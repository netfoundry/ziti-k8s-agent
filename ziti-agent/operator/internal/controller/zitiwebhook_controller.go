/*
Copyright 2025 NetFoundry.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/enroll"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

const zitiWebhookFinalizer = "kubernetes.openziti.io/zitiwebhook"

// ZitiWebhookReconciler reconciles a ZitiWebhook object
type ZitiWebhookReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiWebhook object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("ZitiWebhook Reconciliation started")

	zitiwebhook := &kubernetesv1alpha1.ZitiWebhook{}
	if err := r.Get(ctx, req.NamespacedName, zitiwebhook); err != nil && apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	// Check if the ZitiWebhook is being deleted
	if zitiwebhook.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so register the finalizer if not already present
		if !controllerutil.ContainsFinalizer(zitiwebhook, zitiWebhookFinalizer) {
			controllerutil.AddFinalizer(zitiwebhook, zitiWebhookFinalizer)
			if err := r.Update(ctx, zitiwebhook); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Added finalizer to ZitiWebhook", "ZitiWebhook.Name", zitiwebhook.Name)
			return ctrl.Result{}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(zitiwebhook, zitiWebhookFinalizer) {
			// Our finalizer is present, so lets handle any external dependency
			if err := r.finalizeZitiWebhook(ctx, zitiwebhook); err != nil {
				// If fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// Remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(zitiwebhook, zitiWebhookFinalizer)
			if err := r.Update(ctx, zitiwebhook); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Removed finalizer from ZitiWebhook", "ZitiWebhook.Name", zitiwebhook.Name)
			return ctrl.Result{}, nil
		}
	}

	foundAdminSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-secret",
	}, foundAdminSecret); err != nil && apierrors.IsNotFound(err) {
		if zitiwebhook.Spec.AdminJwt == "" {
			return ctrl.Result{}, errors.New("admin jwt is empty")
		}
		if err := r.updateAdminSecret(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		isOttExpired := verifyOtt(zitiwebhook.Spec.AdminJwt)
		isCertExpired, _ := checkCertExpiration(foundAdminSecret.Data["tls.crt"])
		if isCertExpired && !isOttExpired {
			if err := r.updateAdminSecret(ctx, zitiwebhook, "update"); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	foundIssuer := &certmanagerv1.Issuer{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-ca-issuer",
	}, foundIssuer); err != nil && apierrors.IsNotFound(err) {
		if err := r.updateIssuer(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundWebhookCert := &certmanagerv1.Certificate{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-admission-cert",
	}, foundWebhookCert); err != nil && apierrors.IsNotFound(err) {
		if err := r.updateCertificate(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundService := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-service",
	}, foundService); err != nil && apierrors.IsNotFound(err) {
		if err := r.updateService(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundClusterRoleList := &rbacv1.ClusterRoleList{}
	if err := r.List(ctx, foundClusterRoleList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(foundClusterRoleList.Items) == 0 {
		if err := r.updateClusterRole(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundClusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	if err := r.List(ctx, foundClusterRoleBindingList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(foundClusterRoleBindingList.Items) == 0 {
		if err := r.updateClusterRoleBinding(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundMutatingWebhookConfigurationList := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	if err := r.List(ctx, foundMutatingWebhookConfigurationList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(foundMutatingWebhookConfigurationList.Items) == 0 {
		if err := r.updateMutatingWebhookConfiguration(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	foundWebhookDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-deployment",
	}, foundWebhookDeployment); err != nil && apierrors.IsNotFound(err) {
		if err := r.updateDeployment(ctx, zitiwebhook, "create"); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("ZitiWebhook Reconciliation finished")
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubernetesv1alpha1.ZitiWebhook{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *ZitiWebhookReconciler) updateAdminSecret(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	zitiCfg, err := enrollIdentityWithJwt(zitiwebhook.Spec.AdminJwt)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-secret",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Data: map[string][]byte{
			"tls.key": []byte(strings.TrimPrefix(zitiCfg.ID.Key, "pem:")),
			"tls.crt": []byte(strings.TrimPrefix(zitiCfg.ID.Cert, "pem:")),
			"tls.ca":  []byte(strings.TrimPrefix(zitiCfg.ID.CA, "pem:")),
		},
		Type: "kubernetes.io/tls",
	}
	if err := controllerutil.SetControllerReference(zitiwebhook, secret, r.Scheme); err != nil {
		return err
	}
	if method == "update" {
		if err := r.Client.Update(ctx, secret); err != nil {
			return err
		}
	}
	if method == "create" {
		if err := r.Client.Create(ctx, secret); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateCertificate(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-admission-cert",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName:  zitiwebhook.Spec.Name + "-server-cert",
			Duration:    &metav1.Duration{Duration: 4 * time.Hour},
			RenewBefore: &metav1.Duration{Duration: 1 * time.Hour},
			Subject: &certmanagerv1.X509Subject{
				Organizations: []string{"netfoundry"},
			},
			CommonName: zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc.cluster.local",
			IsCA:       false,
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Encoding:  certmanagerv1.PKCS1,
				Size:      2048,
			},
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageServerAuth,
			},
			DNSNames: []string{
				zitiwebhook.Spec.Name + "-service",
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace,
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc",
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc.cluster.local",
			},
			IssuerRef: certmetav1.ObjectReference{
				Name: zitiwebhook.Spec.Name + "-ca-issuer",
				Kind: "Issuer",
			},
		},
	}
	if err := ctrl.SetControllerReference(zitiwebhook, cert, r.Scheme); err != nil {
		return err
	}
	if method == "create" {
		if err := r.Client.Create(ctx, cert); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateIssuer(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {

	issuer := &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-ca-issuer",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
	if err := ctrl.SetControllerReference(zitiwebhook, issuer, r.Scheme); err != nil {
		return err
	}
	if method == "create" {
		if err := r.Client.Create(ctx, issuer); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateService(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-service",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "https",
					Protocol: corev1.ProtocolTCP,
					Port:     zitiwebhook.Spec.WebhookSpec.ClientConfig.Port,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: zitiwebhook.Spec.DeploymentSpec.Port,
					},
				},
			},
			Selector: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
	}
	if err := controllerutil.SetControllerReference(zitiwebhook, service, r.Scheme); err != nil {
		return err
	}
	if method == "create" {
		if err := r.Client.Create(ctx, service); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateClusterRole(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role",
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: zitiwebhook.Spec.ClusterRoleSpec[0].ApiGroups,
				Resources: zitiwebhook.Spec.ClusterRoleSpec[0].Resources,
				Verbs:     zitiwebhook.Spec.ClusterRoleSpec[0].Verbs,
			},
		},
	}
	if method == "create" {
		if err := r.Client.Create(ctx, clusterRole); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateClusterRoleBinding(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role-binding",
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: zitiwebhook.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     zitiwebhook.Spec.Name + "-cluster-role",
		},
	}
	if method == "create" {
		if err := r.Client.Create(ctx, clusterRoleBinding); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateMutatingWebhookConfiguration(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-mutating-webhook-configuration",
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
			Annotations: map[string]string{
				"cert-manager.io/inject-ca-from": zitiwebhook.Namespace + "/" + zitiwebhook.Spec.Name + "-admission-cert",
			},
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name: "tunnel.ziti.webhook",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      zitiwebhook.Spec.Name + "-service",
						Namespace: zitiwebhook.Namespace,
						Port:      &zitiwebhook.Spec.WebhookSpec.ClientConfig.Port,
						Path:      &zitiwebhook.Spec.WebhookSpec.ClientConfig.Path,
					},
					CABundle: []byte(zitiwebhook.Spec.WebhookSpec.ClientConfig.CaBundle),
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
							admissionregistrationv1.Delete,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"v1", "v1beta1"},
							Resources:   []string{"pods"},
						},
					},
				},
				SideEffects:    &zitiwebhook.Spec.WebhookSpec.SideEffectType,
				TimeoutSeconds: &zitiwebhook.Spec.WebhookSpec.TimeoutSeconds,
				MatchPolicy:    &zitiwebhook.Spec.WebhookSpec.MatchPolicy,
				FailurePolicy:  &zitiwebhook.Spec.WebhookSpec.FailurePolicy,
				AdmissionReviewVersions: []string{
					"v1",
				},
				// ObjectSelector: &metav1.LabelSelector{
				// 	MatchExpressions: []metav1.LabelSelectorRequirement{
				// 		{
				// 			Key:      zitiwebhook.Spec.WebhookSpec.TunnelSelectorKey,
				// 			Operator: metav1.LabelSelectorOpIn,
				// 			Values:   zitiwebhook.Spec.WebhookSpec.SelectorValues,
				// 		},
				// 	},
				// },
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"kube-system"},
						},
						{
							Key:      zitiwebhook.Spec.WebhookSpec.TunnelSelectorKey,
							Operator: metav1.LabelSelectorOpIn,
							Values:   zitiwebhook.Spec.WebhookSpec.SelectorValues,
						},
					},
				},
			},
		},
	}
	if method == "create" {
		if err := r.Client.Create(ctx, mutatingWebhookConfiguration); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) updateDeployment(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook, method string) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-deployment",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zitiwebhook.Spec.DeploymentSpec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                    zitiwebhook.Spec.Name,
					"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    zitiwebhook.Spec.Name,
						"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            zitiwebhook.Spec.Name,
							Image:           zitiwebhook.Spec.DeploymentSpec.Image + ":" + zitiwebhook.Spec.DeploymentSpec.ImageVersion,
							ImagePullPolicy: corev1.PullPolicy(zitiwebhook.Spec.DeploymentSpec.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: zitiwebhook.Spec.DeploymentSpec.Port,
								},
							},
							Args: []string{
								"webhook",
							},
							Env: []corev1.EnvVar{
								{
									Name: "TLS_CERT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.Name + "-server-cert",
											},
											Key: "tls.crt",
										},
									},
								},
								{
									Name: "TLS_PRIVATE_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.Name + "-server-cert",
											},
											Key: "tls.key",
										},
									},
								},
								{
									Name:  "ZITI_MGMT_API",
									Value: zitiwebhook.Spec.DeploymentSpec.ZitiCtrlMgmtApi,
								},
								{
									Name: "ZITI_ADMIN_CERT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.Name + "-secret",
											},
											Key: "tls.crt",
										},
									},
								},
								{
									Name: "ZITI_ADMIN_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.Name + "-secret",
											},
											Key: "tls.key",
										},
									},
								},
								{
									Name: "ZITI_CTRL_CA_BUNDLE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.Name + "-secret",
											},
											Key: "tls.ca",
										},
									},
								},
								{
									Name:  "ZITI_ROLE_KEY",
									Value: zitiwebhook.Spec.DeploymentSpec.ZitiRoleKey,
								},
								{
									Name:  "POD_SECURITY_CONTEXT_OVERRIDE",
									Value: fmt.Sprintf("%t", zitiwebhook.Spec.DeploymentSpec.PodSecurityOverride),
								},
								{
									Name:  "SEARCH_DOMAIN_LIST",
									Value: zitiwebhook.Spec.DeploymentSpec.SearchDomainList,
								},
								{
									Name:  "SIDECAR_IMAGE_VERSION",
									Value: zitiwebhook.Spec.DeploymentSpec.SidecarImageVersion,
								},
							},
						},
					},
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    zitiwebhook.Spec.DeploymentSpec.ResourceLimit["cpu"],
							corev1.ResourceMemory: zitiwebhook.Spec.DeploymentSpec.ResourceLimit["memory"],
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    zitiwebhook.Spec.DeploymentSpec.ResourceRequest["cpu"],
							corev1.ResourceMemory: zitiwebhook.Spec.DeploymentSpec.ResourceRequest["memory"],
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(zitiwebhook, deployment, r.Scheme); err != nil {
		return err
	}
	if method == "create" {
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
	}
	return nil
}

func (r *ZitiWebhookReconciler) finalizeZitiWebhook(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role",
		},
	}
	if err := r.Delete(ctx, clusterRole); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role-binding",
		},
	}
	if err := r.Delete(ctx, clusterRoleBinding); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-server-cert",
			Namespace: zitiwebhook.Namespace,
		},
	}
	if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-mutating-webhook-configuration",
		},
	}
	if err := r.Delete(ctx, mutatingWebhookConfiguration); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func verifyOtt(ott string) bool {
	// Parse the token without verifying signature
	token, _, err := new(jwt.Parser).ParseUnverified(ott, jwt.MapClaims{})
	if err != nil {
		log.Log.Error(err, "Error parsing token:")
		return false
	}

	// Check if the token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		expirationTime := claims["exp"].(float64)
		currentTime := time.Now().Unix()

		if currentTime > int64(expirationTime) {
			return false
		} else {
			return true
		}
	}
	return false
}

func enrollIdentityWithJwt(jwtToken string) (*ziti.Config, error) {
	tkn, _, err := enroll.ParseToken(jwtToken)
	if err != nil {
		return nil, err
	}
	flags := enroll.EnrollmentFlags{
		Token:  tkn,
		KeyAlg: "RSA",
	}
	zitiCfg, err := enroll.Enroll(flags)
	if err != nil {
		return nil, err
	}
	return zitiCfg, nil
}

func checkCertExpiration(certData []byte) (bool, error) {
	cert, err := parseCertificate(certData)
	if err != nil {
		return false, fmt.Errorf("failed to parse certificate: %w", err)
	}

	expirationTime := cert.NotAfter
	currentTime := time.Now()

	if currentTime.After(expirationTime) {
		return false, nil
	} else {
		return true, nil
	}
}

func parseCertificate(certData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM-encoded certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}
