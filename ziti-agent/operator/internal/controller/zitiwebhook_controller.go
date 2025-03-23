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
	"fmt"
	"reflect"
	"strconv"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
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

	actualStateIssuer := &certmanagerv1.Issuer{}
	desiredStateIssuer := r.getDesiredStateIssuer(ctx, zitiwebhook)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-ca-issuer",
	}, actualStateIssuer); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new Issuer", "Issuer.Namespace", desiredStateIssuer.Namespace, "Issuer.Name", desiredStateIssuer.Name)
		log.V(5).Info("Creating a new Issuer", "Issuer.Namespace", desiredStateIssuer.Namespace, "Issuer.Spec", desiredStateIssuer.Spec)
		if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateIssuer, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateIssuer); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if !reflect.DeepEqual(actualStateIssuer.Spec, desiredStateIssuer.Spec) {
			log.V(4).Info("Updating Issuer", "Issuer.Actual", actualStateIssuer.Name, "Issuer.Desired", desiredStateIssuer.Name)
			log.V(5).Info("Updating Issuer", "Issuer.Actual", actualStateIssuer.Spec, "Issuer.Desired", desiredStateIssuer.Spec)
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateIssuer, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateIssuer); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateWebhookCert := &certmanagerv1.Certificate{}
	desiredStateWebhookCert := r.getDesiredStateCertificate(ctx, zitiwebhook)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-admission-cert",
	}, actualStateWebhookCert); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new Certificate", "Certificate.Namespace", desiredStateWebhookCert.Namespace, "Certificate.Name", desiredStateWebhookCert.Name)
		log.V(5).Info("Creating a new Certificate", "Certificate.Namespace", desiredStateWebhookCert.Namespace, "Certificate.Spec", desiredStateWebhookCert.Spec)
		if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateWebhookCert, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateWebhookCert); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if !reflect.DeepEqual(actualStateWebhookCert.Spec, desiredStateWebhookCert.Spec) {
			log.V(4).Info("Updating Certificate", "Certificate.Actual", actualStateWebhookCert.Name, "Certificate.Desired", desiredStateWebhookCert.Name)
			log.V(5).Info("Updating Certificate", "Certificate.Actual", actualStateWebhookCert.Spec, "Certificate.Desired", desiredStateWebhookCert.Spec)
			desiredStateWebhookCert.ObjectMeta.ResourceVersion = actualStateWebhookCert.ObjectMeta.ResourceVersion
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateWebhookCert, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateWebhookCert); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateService := &corev1.Service{}
	desiredStateService := r.getDesiredStateService(ctx, zitiwebhook)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-service",
	}, actualStateService); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new Service", "Service.Namespace", desiredStateService.Namespace, "Service.Name", desiredStateService.Name)
		log.V(5).Info("Creating a new Service", "Service.Namespace", desiredStateService.Namespace, "Service.Spec", desiredStateService.Spec)
		if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateService, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateService); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Normalize desiredStateService to eliminate the difference in assigned IPs
		if actualStateService.Spec.ClusterIP != "" || actualStateService.Spec.ClusterIPs == nil {
			desiredStateService.Spec.ClusterIP = actualStateService.Spec.ClusterIP
			desiredStateService.Spec.ClusterIPs = actualStateService.Spec.ClusterIPs
		}
		if !reflect.DeepEqual(actualStateService.Spec, desiredStateService.Spec) {
			log.V(4).Info("Updating Service", "Service.Actual", actualStateService.Name, "Service.Desired", desiredStateService.Name)
			log.V(5).Info("Updating Service", "Service.Actual", actualStateService.Spec, "Service.Desired", desiredStateService.Spec)
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateService, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateService); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateServiceAccount := &corev1.ServiceAccount{}
	desiredStateServiceAccount := r.getDesiredStateServiceAccount(ctx, zitiwebhook)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-service-account",
	}, actualStateServiceAccount); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", desiredStateServiceAccount.Namespace, "ServiceAccount.Name", desiredStateServiceAccount.Name)
		log.V(5).Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", desiredStateServiceAccount.Namespace, "ServiceAccount.ImagePullSecrets", desiredStateServiceAccount.ImagePullSecrets)
		log.V(5).Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", desiredStateServiceAccount.Namespace, "ServiceAccount.Secrets", desiredStateServiceAccount.Secrets)
		log.V(5).Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", desiredStateServiceAccount.Namespace, "ServiceAccount.AutomountServiceAccountToken", desiredStateServiceAccount.AutomountServiceAccountToken)
		if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateServiceAccount, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateServiceAccount); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if !reflect.DeepEqual(actualStateServiceAccount.ImagePullSecrets, desiredStateServiceAccount.ImagePullSecrets) ||
			!reflect.DeepEqual(actualStateServiceAccount.Secrets, desiredStateServiceAccount.Secrets) ||
			!reflect.DeepEqual(actualStateServiceAccount.AutomountServiceAccountToken, desiredStateServiceAccount.AutomountServiceAccountToken) {
			log.V(4).Info("Updating ServiceAccount", "ServiceAccount.Actual", actualStateServiceAccount.Name, "ServiceAccount.Desired", desiredStateServiceAccount.Name)
			log.V(5).Info("Updating ServiceAccount", "ServiceAccount.Actual", actualStateServiceAccount.ImagePullSecrets, "ServiceAccount.Desired", desiredStateServiceAccount.ImagePullSecrets)
			log.V(5).Info("Updating ServiceAccount", "ServiceAccount.Actual", actualStateServiceAccount.Secrets, "ServiceAccount.Desired", desiredStateServiceAccount.Secrets)
			log.V(5).Info("Updating ServiceAccount", "ServiceAccount.Actual", actualStateServiceAccount.AutomountServiceAccountToken, "ServiceAccount.Desired", desiredStateServiceAccount.AutomountServiceAccountToken)
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateServiceAccount, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateServiceAccount); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateClusterRoleList := &rbacv1.ClusterRoleList{}
	desiredStateClusterRole := r.getDesiredStateClusterRole(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateClusterRoleList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(actualStateClusterRoleList.Items) == 0 {
		log.V(4).Info("Creating a new ClusterRole", "ClusterRole.Namespace", desiredStateClusterRole.Namespace, "ClusterRole.Name", desiredStateClusterRole.Name)
		log.V(5).Info("Creating a new ClusterRole", "ClusterRole.Namespace", desiredStateClusterRole.Namespace, "ClusterRole.Rules", desiredStateClusterRole.Rules)
		if err := r.Create(ctx, desiredStateClusterRole); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if !reflect.DeepEqual(actualStateClusterRoleList.Items[0].Rules, desiredStateClusterRole.Rules) {
			log.V(4).Info("Updating ClusterRole", "ClusterRole.Actual", actualStateClusterRoleList.Items[0].Name, "ClusterRole.Desired", desiredStateClusterRole.Name)
			log.V(5).Info("Updating ClusterRole", "ClusterRole.Actual", actualStateClusterRoleList.Items[0].Rules, "ClusterRole.Desired", desiredStateClusterRole.Rules)
			if err := r.Update(ctx, desiredStateClusterRole); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateClusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	desiredStateClusterRoleBinding := r.getDesiredStateClusterRoleBinding(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateClusterRoleBindingList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(actualStateClusterRoleBindingList.Items) == 0 {
		log.V(4).Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Namespace", desiredStateClusterRoleBinding.Namespace, "ClusterRoleBinding.Name", desiredStateClusterRoleBinding.Name)
		log.V(5).Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Namespace", desiredStateClusterRoleBinding.Namespace, "ClusterRoleBinding.RoleRef", desiredStateClusterRoleBinding.RoleRef)
		log.V(5).Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Namespace", desiredStateClusterRoleBinding.Namespace, "ClusterRoleBinding.Subjects", desiredStateClusterRoleBinding.Subjects)
		if err := r.Create(ctx, desiredStateClusterRoleBinding); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].RoleRef, desiredStateClusterRoleBinding.RoleRef) || !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].Subjects, desiredStateClusterRoleBinding.Subjects) {
			log.V(4).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].Name, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.Name)
			log.V(5).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].RoleRef, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.RoleRef)
			log.V(5).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].Subjects, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.Subjects)
			if err := r.Update(ctx, desiredStateClusterRoleBinding); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateMutatingWebhookConfigurationList := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	desiredStateMutatingWebhookConfiguration := r.getDesiredStateMutatingWebhookConfiguration(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateMutatingWebhookConfigurationList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"app": zitiwebhook.Spec.Name,
			}),
		},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(actualStateMutatingWebhookConfigurationList.Items) == 0 {
		log.V(4).Info("Creating a new MutatingWebhookConfiguration", "MutatingWebhookConfiguration.Namespace", desiredStateMutatingWebhookConfiguration.Namespace, "MutatingWebhookConfiguration.Name", desiredStateMutatingWebhookConfiguration.Name)
		log.V(5).Info("Creating a new MutatingWebhookConfiguration", "MutatingWebhookConfiguration.Namespace", desiredStateMutatingWebhookConfiguration.Namespace, "MutatingWebhookConfiguration.Webhook", desiredStateMutatingWebhookConfiguration.Webhooks[0])
		if err := r.Create(ctx, desiredStateMutatingWebhookConfiguration); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if len(desiredStateMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle) == 0 && len(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0].ClientConfig.CABundle) > 0 {
			desiredStateMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle = actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0].ClientConfig.CABundle
			log.V(5).Info("Desired MutatingWebhookConfiguration", "CA BUndled", desiredStateMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle)
		}
		desiredStateMutatingWebhookConfiguration.ObjectMeta.ResourceVersion = actualStateMutatingWebhookConfigurationList.Items[0].ObjectMeta.ResourceVersion
		log.V(5).Info("Desired MutatingWebhookConfiguration", "ResourceVersion", desiredStateMutatingWebhookConfiguration.ObjectMeta.ResourceVersion)
		if !reflect.DeepEqual(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0], desiredStateMutatingWebhookConfiguration.Webhooks[0]) {
			log.V(4).Info("Updating MutatingWebhookConfiguration", "MutatingWebhookConfiguration.Actual", actualStateMutatingWebhookConfigurationList.Items[0].Name, "MutatingWebhookConfiguration.Desired", desiredStateMutatingWebhookConfiguration.Name)
			log.V(5).Info("Updating MutatingWebhookConfiguration", "MutatingWebhookConfiguration.Actual", actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0], "MutatingWebhookConfiguration.Desired", desiredStateMutatingWebhookConfiguration.Webhooks[0])
			if err := r.Update(ctx, desiredStateMutatingWebhookConfiguration); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	actualStateWebhookDeployment := &appsv1.Deployment{}
	desiredStateWebhookDeployment := r.getDesiredStateDeploymentConfiguration(ctx, zitiwebhook)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitiwebhook.Namespace,
		Name:      zitiwebhook.Spec.Name + "-deployment",
	}, actualStateWebhookDeployment); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new Deployment", "Deployment.Namespace", desiredStateWebhookDeployment.Namespace, "Deployment.Name", desiredStateWebhookDeployment.Name)
		log.V(5).Info("Creating a new Deployment", "Deployment.Namespace", desiredStateWebhookDeployment.Namespace, "Deployment.Spec", desiredStateWebhookDeployment.Spec)
		if err := ctrl.SetControllerReference(zitiwebhook, desiredStateWebhookDeployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateWebhookDeployment); err != nil {
			return ctrl.Result{}, err
		}
	} else if !reflect.DeepEqual(actualStateWebhookDeployment.Spec, desiredStateWebhookDeployment.Spec) {
		log.V(4).Info("Updating Deployment", "Deployment.Actual", actualStateWebhookDeployment.Name, "Deployment.Desired", desiredStateWebhookDeployment.Name)
		log.V(5).Info("Updating Deployment", "Deployment.Actual", actualStateWebhookDeployment.Spec, "Deployment.Desired", desiredStateWebhookDeployment.Spec)
		if err := ctrl.SetControllerReference(zitiwebhook, desiredStateWebhookDeployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Update(ctx, desiredStateWebhookDeployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("ZitiWebhook Reconciliation finished")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubernetesv1alpha1.ZitiWebhook{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&certmanagerv1.Certificate{}).
		Owns(&certmanagerv1.Issuer{}).
		Complete(r)
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

func (r *ZitiWebhookReconciler) getDesiredStateIssuer(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *certmanagerv1.Issuer {
	_ = log.FromContext(ctx)
	return &certmanagerv1.Issuer{
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
}

func (r *ZitiWebhookReconciler) getDesiredStateCertificate(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *certmanagerv1.Certificate {
	_ = log.FromContext(ctx)
	return &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-admission-cert",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName: zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc.cluster.local",
			DNSNames: []string{
				zitiwebhook.Spec.Name + "-service",
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace,
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc",
				zitiwebhook.Spec.Name + "-service." + zitiwebhook.Namespace + ".svc.cluster.local",
			},
			Duration: &metav1.Duration{Duration: time.Duration(zitiwebhook.Spec.Cert.Duration) * time.Hour},
			IssuerRef: certmetav1.ObjectReference{
				Name: zitiwebhook.Spec.Name + "-ca-issuer",
				Kind: "Issuer",
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Encoding:  certmanagerv1.PKCS1,
				Size:      2048,
			},
			RenewBefore: &metav1.Duration{Duration: time.Duration(zitiwebhook.Spec.Cert.RenewBefore) * time.Hour},
			SecretName:  zitiwebhook.Spec.Name + "-server-cert",
			Subject: &certmanagerv1.X509Subject{
				Organizations: zitiwebhook.Spec.Cert.Organizations,
			},
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageServerAuth,
			},
		},
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateService(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *corev1.Service {
	_ = log.FromContext(ctx)
	cluster := corev1.ServiceInternalTrafficPolicyCluster
	singleStack := corev1.IPFamilyPolicySingleStack
	return &corev1.Service{
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
					Port:     zitiwebhook.Spec.MutatingWebhookSpec.ClientConfig.Port,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: zitiwebhook.Spec.DeploymentSpec.Port,
					},
				},
			},
			InternalTrafficPolicy: &cluster,
			IPFamilies:            []corev1.IPFamily{corev1.IPv4Protocol},
			IPFamilyPolicy:        &singleStack,
			Selector: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
		},
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateServiceAccount(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *corev1.ServiceAccount {
	_ = log.FromContext(ctx)
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-service-account",
			Namespace: zitiwebhook.Namespace,
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		ImagePullSecrets:             zitiwebhook.Spec.ServiceAccount.ImagePullSecrets,
		Secrets:                      zitiwebhook.Spec.ServiceAccount.Secrets,
		AutomountServiceAccountToken: zitiwebhook.Spec.ServiceAccount.AutomountServiceAccountToken,
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateClusterRole(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *rbacv1.ClusterRole {
	_ = log.FromContext(ctx)
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role",
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		Rules: zitiwebhook.Spec.ClusterRoleSpec.Rules,
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateClusterRoleBinding(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *rbacv1.ClusterRoleBinding {
	_ = log.FromContext(ctx)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: zitiwebhook.Spec.Name + "-cluster-role-binding",
			Labels: map[string]string{
				"app":                    zitiwebhook.Spec.Name,
				"app.kubernetes.io/name": zitiwebhook.Spec.Name + "-" + zitiwebhook.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     zitiwebhook.Spec.Name + "-cluster-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      zitiwebhook.Spec.Name + "-service-account",
				Namespace: zitiwebhook.Namespace,
			},
		},
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateMutatingWebhookConfiguration(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *admissionregistrationv1.MutatingWebhookConfiguration {
	_ = log.FromContext(ctx)
	return &admissionregistrationv1.MutatingWebhookConfiguration{
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
						Port:      &zitiwebhook.Spec.MutatingWebhookSpec.ClientConfig.Port,
						Path:      &zitiwebhook.Spec.MutatingWebhookSpec.ClientConfig.Path,
					},
					CABundle: []byte(zitiwebhook.Spec.MutatingWebhookSpec.ClientConfig.CaBundle),
				},
				Rules:                   zitiwebhook.Spec.MutatingWebhookSpec.Rules,
				ObjectSelector:          zitiwebhook.Spec.MutatingWebhookSpec.ObjectSelector,
				NamespaceSelector:       zitiwebhook.Spec.MutatingWebhookSpec.NamespaceSelector,
				SideEffects:             zitiwebhook.Spec.MutatingWebhookSpec.SideEffectType,
				TimeoutSeconds:          zitiwebhook.Spec.MutatingWebhookSpec.TimeoutSeconds,
				MatchPolicy:             zitiwebhook.Spec.MutatingWebhookSpec.MatchPolicy,
				FailurePolicy:           zitiwebhook.Spec.MutatingWebhookSpec.FailurePolicy,
				AdmissionReviewVersions: zitiwebhook.Spec.MutatingWebhookSpec.AdmissionReviewVersions,
				ReinvocationPolicy:      zitiwebhook.Spec.MutatingWebhookSpec.ReinvocationPolicy,
			},
		},
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateDeploymentConfiguration(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *appsv1.Deployment {
	_ = log.FromContext(ctx)
	return &appsv1.Deployment{
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
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Args: []string{
								"webhook",
								"--v=" + strconv.FormatInt(int64(zitiwebhook.Spec.DeploymentSpec.LogLevel), 10),
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
									Name: "ZITI_ADMIN_CERT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitiwebhook.Spec.ZitiControllerName + "-secret",
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
												Name: zitiwebhook.Spec.ZitiControllerName + "-secret",
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
												Name: zitiwebhook.Spec.ZitiControllerName + "-secret",
											},
											Key: "tls.ca",
										},
									},
								},
								{
									Name:  "SIDECAR_IMAGE",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.SidecarImage,
								},
								{
									Name:  "SIDECAR_IMAGE_VERSION",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.SidecarImageVersion,
								},
								{
									Name:  "SIDECAR_IMAGE_PULL_POLICY",
									Value: string(zitiwebhook.Spec.DeploymentSpec.Env.SidecarImagePullPolicy),
								},
								{
									Name:  "SIDECAR_PREFIX",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.SidecarPrefix,
								},
								{
									Name:  "SIDECAR_IDENTITY_DIR",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.SidecarIdentityDir,
								},
								{
									Name:  "ZITI_MGMT_API",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi,
								},
								{
									Name:  "POD_SECURITY_CONTEXT_OVERRIDE",
									Value: fmt.Sprintf("%t", zitiwebhook.Spec.DeploymentSpec.Env.PodSecurityOverride),
								},
								{
									Name:  "CLUSTER_DNS_SERVICE_IP",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.ClusterDnsServiceIP,
								},
								{
									Name:  "SEARCH_DOMAIN_LIST",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.SearchDomainList,
								},
								{
									Name:  "ZITI_ROLE_KEY",
									Value: zitiwebhook.Spec.DeploymentSpec.Env.ZitiRoleKey,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    zitiwebhook.Spec.DeploymentSpec.ResourceLimit["cpu"],
									corev1.ResourceMemory: zitiwebhook.Spec.DeploymentSpec.ResourceLimit["memory"],
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    zitiwebhook.Spec.DeploymentSpec.ResourceRequest["cpu"],
									corev1.ResourceMemory: zitiwebhook.Spec.DeploymentSpec.ResourceRequest["memory"],
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					ServiceAccountName:            zitiwebhook.Spec.Name + "-service-account",
					DeprecatedServiceAccount:      zitiwebhook.Spec.Name + "-service-account",
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: &zitiwebhook.Spec.DeploymentSpec.TerminationGracePeriodSeconds,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: zitiwebhook.Spec.DeploymentSpec.MaxUnavailable,
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: zitiwebhook.Spec.DeploymentSpec.MaxSurge,
					},
				},
			},
			ProgressDeadlineSeconds: &zitiwebhook.Spec.DeploymentSpec.ProgressDeadlineSeconds,
			RevisionHistoryLimit:    &zitiwebhook.Spec.DeploymentSpec.RevisionHistoryLimit,
		},
	}
}
