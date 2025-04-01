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
	"github.com/golang-jwt/jwt/v5"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"slices"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

const zitiWebhookFinalizer = "kubernetes.openziti.io/zitiwebhook"

// ZitiWebhookReconciler reconciles a ZitiWebhook object
type ZitiWebhookReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ZitiControllerChan   chan kubernetesv1alpha1.ZitiController
	CachedZitiController *kubernetesv1alpha1.ZitiController
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
	log.V(2).Info("ZitiWebhook Reconciliation started")

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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Added finalizer to ZitiWebhook")
			log.V(5).Info("Added finalizer to ZitiWebhook", "ZitiWebhook.Name", zitiwebhook.Name)
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Removed", "Removed finalizer from ZitiWebhook")
			return ctrl.Result{}, nil
		}
	}

	// Merge defaults and ziticontroller specs if changes are detected
	log.V(5).Info("ZitiWebhook Actual", "Name", zitiwebhook.Name, "Specs", zitiwebhook.Spec)
	defaultSpecs := zitiwebhook.GetDefaults()
	log.V(5).Info("ZitiWebhook Default", "Name", zitiwebhook.Name, "Specs", defaultSpecs)
	err, ok := r.mergeSpecs(ctx, &zitiwebhook.Spec, defaultSpecs)
	if err == nil && ok {
		select {
		case ziticontroller := <-r.ZitiControllerChan:
			log.V(5).Info("ZitiController Spec", "Name", ziticontroller.Spec.Name, "ZitiController.Spec", ziticontroller.Spec)
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Update", "Using ZitiController from channel")
			r.CachedZitiController = &ziticontroller
			zitiwebhook.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
			if zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi == "" {
				zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi, _ = getZitiControllerUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
				zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi = zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi + "/edge/management/v1"
				log.V(5).Info("ZitiController URL", "ZitiCtrlMgmtApi", zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi)
			}
		default:
			if r.CachedZitiController != nil {
				log.V(5).Info("Cached ZitiController Spec", "Name", r.CachedZitiController.Spec.Name, "ZitiController.Spec", r.CachedZitiController.Spec)
				zitiwebhook.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
				if zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi == "" {
					zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi, _ = getZitiControllerUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
					zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi = zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi + "/edge/management/v1"
					log.V(5).Info("ZitiController URL", "ZitiCtrlMgmtApi", zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi)
				}
			} else {
				log.V(5).Info("No ZitiController Spec available")
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "No ZitiController Spec available")
			}
		}
		if err := r.Update(ctx, zitiwebhook); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Merged", "Merged default specs to ZitiWebhook")
		log.V(5).Info("ZitiWebhook Merged", "Name", zitiwebhook.Name, "Specs", zitiwebhook.Spec)
	} else if err != nil {
		return ctrl.Result{}, err
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create Issuer")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new Issuer")
	} else {
		if !reflect.DeepEqual(actualStateIssuer.Spec, desiredStateIssuer.Spec) {
			log.V(4).Info("Updating Issuer", "Issuer.Actual", actualStateIssuer.Name, "Issuer.Desired", desiredStateIssuer.Name)
			log.V(5).Info("Updating Issuer", "Issuer.Actual", actualStateIssuer.Spec, "Issuer.Desired", desiredStateIssuer.Spec)
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateIssuer, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateIssuer); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update Issuer")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated Issuer")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create Certificate")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new Certificate")
	} else {
		if !reflect.DeepEqual(actualStateWebhookCert.Spec, desiredStateWebhookCert.Spec) {
			log.V(4).Info("Updating Certificate", "Certificate.Actual", actualStateWebhookCert.Name, "Certificate.Desired", desiredStateWebhookCert.Name)
			log.V(5).Info("Updating Certificate", "Certificate.Actual", actualStateWebhookCert.Spec, "Certificate.Desired", desiredStateWebhookCert.Spec)
			desiredStateWebhookCert.ObjectMeta.ResourceVersion = actualStateWebhookCert.ObjectMeta.ResourceVersion
			if err := controllerutil.SetControllerReference(zitiwebhook, desiredStateWebhookCert, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(ctx, desiredStateWebhookCert); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update Certificate")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated Certificate")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create Service")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new Service")
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
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update Service")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated Service")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create ServiceAccount")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new ServiceAccount")
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
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update ServiceAccount")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated ServiceAccount")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create ClusterRole")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new ClusterRole")
	} else {
		if !reflect.DeepEqual(actualStateClusterRoleList.Items[0].Rules, desiredStateClusterRole.Rules) {
			log.V(4).Info("Updating ClusterRole", "ClusterRole.Actual", actualStateClusterRoleList.Items[0].Name, "ClusterRole.Desired", desiredStateClusterRole.Name)
			log.V(5).Info("Updating ClusterRole", "ClusterRole.Actual", actualStateClusterRoleList.Items[0].Rules, "ClusterRole.Desired", desiredStateClusterRole.Rules)
			if err := r.Update(ctx, desiredStateClusterRole); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update ClusterRole")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated ClusterRole")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create ClusterRoleBinding")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new ClusterRoleBinding")
	} else {
		if !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].RoleRef, desiredStateClusterRoleBinding.RoleRef) || !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].Subjects, desiredStateClusterRoleBinding.Subjects) {
			log.V(4).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].Name, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.Name)
			log.V(5).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].RoleRef, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.RoleRef)
			log.V(5).Info("Updating ClusterRoleBinding", "ClusterRoleBinding.Actual", actualStateClusterRoleBindingList.Items[0].Subjects, "ClusterRoleBinding.Desired", desiredStateClusterRoleBinding.Subjects)
			if err := r.Update(ctx, desiredStateClusterRoleBinding); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update ClusterRoleBinding")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated ClusterRoleBinding")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create MutatingWebhookConfiguration")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new MutatingWebhookConfiguration")
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
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update MutatingWebhookConfiguration")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated MutatingWebhookConfiguration")
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
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to set controller reference")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateWebhookDeployment); err != nil {
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to create Deployment")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Created", "Created a new Deployment")
	} else if !reflect.DeepEqual(actualStateWebhookDeployment.Spec, desiredStateWebhookDeployment.Spec) {
		log.V(4).Info("Updating Deployment", "Deployment.Actual", actualStateWebhookDeployment.Name, "Deployment.Desired", desiredStateWebhookDeployment.Name)
		log.V(5).Info("Updating Deployment", "Deployment.Actual", actualStateWebhookDeployment.Spec, "Deployment.Desired", desiredStateWebhookDeployment.Spec)
		if err := ctrl.SetControllerReference(zitiwebhook, desiredStateWebhookDeployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Update(ctx, desiredStateWebhookDeployment); err != nil {
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update Deployment")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Updated", "Updated Deployment")
	}

	// Re-fetch the ZitiWebhook object before updating the status
	if err := r.Get(ctx, req.NamespacedName, zitiwebhook); err == nil {
		// Create a copy *before* modifying the status
		existing := zitiwebhook.DeepCopy()
		// Update the status
		zitiwebhook.Status.DeploymentConditions = convertDeploymentConditions(actualStateWebhookDeployment.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.DeploymentConditions)
		zitiwebhook.Status.IssuerConditions = convertIssuerConditions(actualStateIssuer.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.IssuerConditions)
		zitiwebhook.Status.CertificateConditions = convertCertificateConditions(actualStateWebhookCert.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.CertificateConditions)
		// Attempt to patch the status
		if err := r.Status().Patch(ctx, zitiwebhook, client.MergeFrom(existing)); err != nil {
			log.Error(err, "Failed to patch ZitiWebhook status")
			r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to update ZitiWebhook status")
			return ctrl.Result{}, err
		}
	} else {
		r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to get ZitiWebhook")
		return ctrl.Result{}, err
	}

	log.V(2).Info("ZitiWebhook Reconciliation finished")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("zitiwebhook-controller")
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

func (r *ZitiWebhookReconciler) mergeSpecs(ctx context.Context, current, desired any) (error, bool) {
	log := log.FromContext(ctx)
	ok := false
	log.V(5).Info("Merging Structs", "Current", current, "Desired", desired)
	currentVal := reflect.ValueOf(current)
	desiredVal := reflect.ValueOf(desired)

	// Check if the values are pointers; if not, get a pointer to them
	if currentVal.Kind() != reflect.Ptr {
		log.V(5).Info("Current is not a pointer, creating a pointer to it")
		currentPtr := reflect.New(currentVal.Type())
		currentPtr.Elem().Set(currentVal)
		currentVal = currentPtr
	}
	if desiredVal.Kind() != reflect.Ptr {
		log.V(5).Info("Desired is not a pointer, creating a pointer to it")
		desiredPtr := reflect.New(desiredVal.Type())
		desiredPtr.Elem().Set(desiredVal)
		desiredVal = desiredPtr
	}

	currentValElem := currentVal.Elem()
	desiredValElem := desiredVal.Elem()

	for i := range currentValElem.NumField() {
		fieldType := currentValElem.Type().Field(i)
		currentField := currentValElem.Field(i)
		desiredField := desiredValElem.Field(i)
		log.V(5).Info("Setting fields", "Field", fieldType.Name, "Value", currentField)
		log.V(5).Info("Setting fields", "Field", fieldType.Name, "Value", desiredField)
		if r.isManagedField(ctx, fieldType.Name) {
			log.V(5).Info("IsMangedField", "Field", fieldType.Name, "Value", currentField)
			log.V(5).Info("IsMangedField", "Field", fieldType.Name, "Value", desiredField)
			if r.isZeroValue(ctx, currentField) && !r.isZeroValue(ctx, desiredField) {
				if currentField.CanSet() {
					currentField.Set(desiredField)
					ok = true
					log.V(4).Info("CurrentField Set", "Field", fieldType.Name, "Value", currentField.Interface())
				} else {
					return fmt.Errorf("cannot set field %s", fieldType.Name), ok
				}
			}
		}
	}
	return nil, ok
}

func (r *ZitiWebhookReconciler) isManagedField(ctx context.Context, fieldName string) bool {
	_ = log.FromContext(ctx)
	managedFields := []string{"Name", "ZitiControllerName", "Cert", "DeploymentSpec", "MutatingWebhookSpec", "ClusterRoleSpec", "ServiceAccount", "Revision"}
	return slices.Contains(managedFields, fieldName)
}

func (r *ZitiWebhookReconciler) isZeroValue(ctx context.Context, field reflect.Value) bool {
	_ = log.FromContext(ctx)
	return reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface())
}

func convertDeploymentConditions(conds []appsv1.DeploymentCondition) []appsv1.DeploymentCondition {
	result := make([]appsv1.DeploymentCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, appsv1.DeploymentCondition{
			Type:               appsv1.DeploymentConditionType(c.Type),
			Status:             corev1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			LastUpdateTime:     c.LastUpdateTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func convertIssuerConditions(conds []certmanagerv1.IssuerCondition) []certmanagerv1.IssuerCondition {
	result := make([]certmanagerv1.IssuerCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, certmanagerv1.IssuerCondition{
			Type:               certmanagerv1.IssuerConditionType(c.Type),
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func convertCertificateConditions(conds []certmanagerv1.CertificateCondition) []certmanagerv1.CertificateCondition {
	result := make([]certmanagerv1.CertificateCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, certmanagerv1.CertificateCondition{
			Type:               certmanagerv1.CertificateConditionType(c.Type),
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func decodeJWT(tokenString string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims from JWT")
	}

	return claims, nil
}

func getZitiControllerUrlFromJwt(tokenString string) (string, error) {

	claims, err := decodeJWT(tokenString)
	if err != nil {
		return "", err
	}
	issuer, err := claims.GetIssuer()
	if err != nil {
		return "", err
	}
	return issuer, nil
}
