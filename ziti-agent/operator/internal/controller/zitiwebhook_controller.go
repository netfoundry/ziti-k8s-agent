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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/internal/utils"
)

const zitiWebhookFinalizer = "kubernetes.openziti.io/zitiwebhook"

// ZitiWebhookReconciler reconciles a ZitiWebhook object
type ZitiWebhookReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ZitiControllerChan   chan *kubernetesv1alpha1.ZitiController
	CachedZitiController *kubernetesv1alpha1.ZitiController
}

// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitiwebhooks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

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
	err, ok := utils.MergeSpecs(ctx, &zitiwebhook.Spec, defaultSpecs)
	if err == nil && ok {
		select {
		case ziticontroller := <-r.ZitiControllerChan:
			log.V(5).Info("ZitiController Spec", "Name", ziticontroller.Spec.Name, "ZitiController.Spec", ziticontroller.Spec)
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Update", "Using ZitiController from channel")
			r.CachedZitiController = ziticontroller
			zitiwebhook.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
			zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi = r.CachedZitiController.Spec.ZitiCtrlMgmtApi
			if zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi == "" {
				zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi, _ = utils.GetUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
				zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi = zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi + "/edge/management/v1"
				log.V(5).Info("ZitiController URL", "ZitiCtrlMgmtApi", zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi)
			}
		default:
			if r.CachedZitiController != nil {
				log.V(5).Info("Cached ZitiController Spec", "Name", r.CachedZitiController.Spec.Name, "ZitiController.Spec", r.CachedZitiController.Spec)
				zitiwebhook.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
				zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi = r.CachedZitiController.Spec.ZitiCtrlMgmtApi
				if zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi == "" {
					zitiwebhook.Spec.DeploymentSpec.Env.ZitiCtrlMgmtApi, _ = utils.GetUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
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
		log.V(2).Info("ZitiWebhook Merged", "Name", zitiwebhook.Name, "Specs", zitiwebhook.Spec)
	} else if err != nil {
		log.V(5).Info("ZitiWebhook Spec merge failed", "Name", zitiwebhook.Name, "Error", err)
		log.V(5).Info("ZitiWebhook Spec merge failed", "Name", zitiwebhook.Name, "Ok is", ok)
		r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "ZitiWebhook Spec merge failed")
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
	} else if err == nil {
		existingIssuerForPatch := actualStateIssuer.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateIssuer.ObjectMeta.Labels, desiredStateIssuer.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "Issuer.Name", actualStateIssuer.Name)
			actualStateIssuer.ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateIssuer, zitiwebhook) {
			log.V(4).Info("Ownership missing, preparing patch", "Issuer.Name", actualStateIssuer.Name)
			if err := controllerutil.SetControllerReference(zitiwebhook, actualStateIssuer, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual Issuer for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateIssuer.Spec, desiredStateIssuer.Spec) {
			log.V(4).Info("Spec differs, preparing patch", "Issuer.Name", actualStateIssuer.Name)
			actualStateIssuer.Spec = desiredStateIssuer.Spec
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to Issuer", "Issuer.Name", actualStateIssuer.Name)
			if err := r.Patch(ctx, actualStateIssuer, client.MergeFrom(existingIssuerForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch Issuer")
				log.Error(err, "Failed to patch Issuer")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched Issuer")
		} else {
			log.V(4).Info("Issuer is up to date", "Issuer.Name", actualStateIssuer.Name)
		}
	} else {
		log.Error(err, "Failed to get Issuer")
		return ctrl.Result{}, err
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
	} else if err == nil {
		existingWebhookCertForPatch := actualStateWebhookCert.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateWebhookCert.ObjectMeta.Labels, desiredStateWebhookCert.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "Certificate.Name", actualStateWebhookCert.Name)
			actualStateWebhookCert.ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateWebhookCert, zitiwebhook) {
			log.V(4).Info("Ownership missing, preparing patch", "Certificate.Name", actualStateWebhookCert.Name)
			if err := controllerutil.SetControllerReference(zitiwebhook, actualStateWebhookCert, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual Certificate for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateWebhookCert.Spec, desiredStateWebhookCert.Spec) {
			log.V(4).Info("Spec differs, preparing patch", "Certificate.Name", actualStateWebhookCert.Name)
			actualStateWebhookCert.Spec = desiredStateWebhookCert.Spec
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to Certificate", "Certificate.Name", actualStateWebhookCert.Name)
			if err := r.Patch(ctx, actualStateWebhookCert, client.MergeFrom(existingWebhookCertForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch Certificate")
				log.Error(err, "Failed to patch Certificate")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched Certificate")
		} else {
			log.V(4).Info("Certificate is up to date", "Certificate.Name", actualStateWebhookCert.Name)
		}
	} else {
		log.Error(err, "Failed to get Certificate")
		return ctrl.Result{}, err
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
	} else if err == nil {
		existingServicetForPatch := actualStateService.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateService.ObjectMeta.Labels, desiredStateService.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "Service.Name", actualStateService.Name)
			actualStateService.ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateService, zitiwebhook) {
			log.V(4).Info("Ownership missing, preparing patch", "Service.Name", actualStateService.Name)
			if err := controllerutil.SetControllerReference(zitiwebhook, actualStateService, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual Service for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		// Normalize desiredStateService to eliminate the difference in assigned IPs
		if actualStateService.Spec.ClusterIP != "" || actualStateService.Spec.ClusterIPs == nil {
			desiredStateService.Spec.ClusterIP = actualStateService.Spec.ClusterIP
			desiredStateService.Spec.ClusterIPs = actualStateService.Spec.ClusterIPs
		}

		if !reflect.DeepEqual(actualStateService.Spec, desiredStateService.Spec) {
			log.V(4).Info("Spec differs, preparing patch", "Service.Name", actualStateService.Name)
			actualStateService.Spec = desiredStateService.Spec
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to Service", "Service.Name", actualStateService.Name)
			if err := r.Patch(ctx, actualStateService, client.MergeFrom(existingServicetForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch Service")
				log.Error(err, "Failed to patch Service")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched Service")
		} else {
			log.V(4).Info("Service is up to date", "Service.Name", actualStateService.Name)
		}
	} else {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
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
	} else if err == nil {
		existingServiceAccountForPatch := actualStateServiceAccount.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateServiceAccount.ObjectMeta.Labels, desiredStateServiceAccount.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "ServiceAccount.Name", actualStateServiceAccount.Name)
			actualStateServiceAccount.ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
		}

		if !metav1.IsControlledBy(actualStateServiceAccount, zitiwebhook) {
			log.V(4).Info("Ownership missing, preparing patch", "ServiceAccount.Name", actualStateServiceAccount.Name)
			if err := controllerutil.SetControllerReference(zitiwebhook, actualStateServiceAccount, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual ServiceAccount for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateServiceAccount.ImagePullSecrets, desiredStateServiceAccount.ImagePullSecrets) ||
			!reflect.DeepEqual(actualStateServiceAccount.Secrets, desiredStateServiceAccount.Secrets) ||
			!reflect.DeepEqual(actualStateServiceAccount.AutomountServiceAccountToken, desiredStateServiceAccount.AutomountServiceAccountToken) {
			log.V(4).Info("Spec differs, preparing patch", "ServiceAccount.Name", actualStateServiceAccount.Name)
			actualStateServiceAccount.ImagePullSecrets = desiredStateServiceAccount.ImagePullSecrets
			actualStateServiceAccount.Secrets = desiredStateServiceAccount.Secrets
			actualStateServiceAccount.AutomountServiceAccountToken = desiredStateServiceAccount.AutomountServiceAccountToken
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to ServiceAccount", "ServiceAccount.Name", actualStateServiceAccount.Name)
			if err := r.Patch(ctx, actualStateServiceAccount, client.MergeFrom(existingServiceAccountForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch ServiceAccount")
				log.Error(err, "Failed to patch ServiceAccount")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched ServiceAccount")
		} else {
			log.V(4).Info("ServiceAccount is up to date", "ServiceAccount.Name", actualStateServiceAccount.Name)
		}
	} else {
		log.Error(err, "Failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	actualStateClusterRoleList := &rbacv1.ClusterRoleList{}
	desiredStateClusterRole := r.getDesiredStateClusterRole(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateClusterRoleList,
		&client.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"metadata.name": zitiwebhook.Spec.Name + "-cluster-role",
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
	} else if err == nil {
		existingCRForPatch := actualStateClusterRoleList.Items[0].DeepCopy()
		needsPatch := false

		// Note: Order matters in DeepEqual for slices. Ensure desiredState always generates rules in a consistent order.
		if !reflect.DeepEqual(actualStateClusterRoleList.Items[0].Rules, desiredStateClusterRole.Rules) {
			log.V(4).Info("Rules differ, preparing patch", "ClusterRole.Name", actualStateClusterRoleList.Items[0].Name)
			actualStateClusterRoleList.Items[0].Rules = desiredStateClusterRole.Rules
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateClusterRoleList.Items[0].ObjectMeta.Labels, desiredStateClusterRole.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "ClusterRole.Name", actualStateClusterRoleList.Items[0].Name)
			actualStateClusterRoleList.Items[0].ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to ClusterRole", "Name", actualStateClusterRoleList.Items[0].Name)
			if err := r.Patch(ctx, &actualStateClusterRoleList.Items[0], client.MergeFrom(existingCRForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch ClusterRole")
				log.Error(err, "Failed to patch ClusterRole")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched ClusterRole")
		} else {
			log.V(4).Info("ClusterRole is up to date", "Name", actualStateClusterRoleList.Items[0].Name)
		}
	} else {
		// Handle other Get errors
		log.Error(err, "Failed to get ClusterRole")
		return ctrl.Result{}, err
	}

	actualStateClusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	desiredStateClusterRoleBinding := r.getDesiredStateClusterRoleBinding(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateClusterRoleBindingList,
		&client.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"metadata.name": zitiwebhook.Spec.Name + "-cluster-role-binding",
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
	} else if err == nil {
		existingCRBForPatch := actualStateClusterRoleBindingList.Items[0].DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].RoleRef, desiredStateClusterRoleBinding.RoleRef) {
			log.V(4).Info("RoleRef differs, preparing patch", "ClusterRoleBinding.Name", actualStateClusterRoleBindingList.Items[0].Name)
			actualStateClusterRoleBindingList.Items[0].RoleRef = desiredStateClusterRoleBinding.RoleRef
			needsPatch = true
		}

		// Note: Order matters in DeepEqual for slices. Ensure desiredState always generates subjects in a consistent order.
		if !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].Subjects, desiredStateClusterRoleBinding.Subjects) {
			log.V(4).Info("Subjects differ, preparing patch", "ClusterRoleBinding.Name", actualStateClusterRoleBindingList.Items[0].Name)
			actualStateClusterRoleBindingList.Items[0].Subjects = desiredStateClusterRoleBinding.Subjects
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateClusterRoleBindingList.Items[0].ObjectMeta.Labels, desiredStateClusterRoleBinding.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "ClusterRoleBinding.Name", actualStateClusterRoleBindingList.Items[0].Name)
			actualStateClusterRoleBindingList.Items[0].ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to ClusterRoleBinding", "Name", actualStateClusterRoleBindingList.Items[0].Name)
			if err := r.Patch(ctx, &actualStateClusterRoleBindingList.Items[0], client.MergeFrom(existingCRBForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch ClusterRoleBinding")
				log.Error(err, "Failed to patch ClusterRoleBinding")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched ClusterRoleBinding")
		} else {
			log.V(4).Info("ClusterRoleBinding is up to date", "Name", actualStateClusterRoleBindingList.Items[0].Name)
		}
	} else {
		log.Error(err, "Failed to get ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	actualStateMutatingWebhookConfigurationList := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	desiredStateMutatingWebhookConfiguration := r.getDesiredStateMutatingWebhookConfiguration(ctx, zitiwebhook)
	if err := r.List(ctx, actualStateMutatingWebhookConfigurationList,
		&client.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"metadata.name": zitiwebhook.Spec.Name + "-mutating-webhook-configuration",
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
	} else if err == nil {
		existingWebhookForPatch := actualStateMutatingWebhookConfigurationList.Items[0].DeepCopy()
		needsPatch := false
		if len(desiredStateMutatingWebhookConfiguration.Webhooks) > 0 && len(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks) > 0 {
			if len(desiredStateMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle) == 0 && len(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0].ClientConfig.CABundle) > 0 {
				log.V(4).Info("Preserving existing CABundle for MutatingWebhookConfiguration")
				desiredStateMutatingWebhookConfiguration.Webhooks[0].ClientConfig.CABundle = actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0].ClientConfig.CABundle
			}
		} else {
			log.V(4).Info("Warning: Webhooks slice is unexpectedly empty for MutatingWebhookConfiguration", "DesiredLen", len(desiredStateMutatingWebhookConfiguration.Webhooks), "ActualLen", len(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks))
			// TODO: Handle this case appropriately, maybe return an error or skip comparison
		}

		if len(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks) != 1 || len(desiredStateMutatingWebhookConfiguration.Webhooks) != 1 ||
			!reflect.DeepEqual(actualStateMutatingWebhookConfigurationList.Items[0].Webhooks[0], desiredStateMutatingWebhookConfiguration.Webhooks[0]) {
			log.V(4).Info("Webhooks differ, preparing patch", "MutatingWebhookConfiguration.Name", actualStateMutatingWebhookConfigurationList.Items[0].Name)
			actualStateMutatingWebhookConfigurationList.Items[0].Webhooks = desiredStateMutatingWebhookConfiguration.Webhooks
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateMutatingWebhookConfigurationList.Items[0].ObjectMeta.Labels, desiredStateMutatingWebhookConfiguration.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "MutatingWebhookConfiguration.Name", actualStateMutatingWebhookConfigurationList.Items[0].Name)
			actualStateMutatingWebhookConfigurationList.Items[0].ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateMutatingWebhookConfigurationList.Items[0].ObjectMeta.Annotations, desiredStateMutatingWebhookConfiguration.ObjectMeta.Annotations) {
			log.V(4).Info("Annotations differ, preparing patch", "MutatingWebhookConfiguration.Name", actualStateMutatingWebhookConfigurationList.Items[0].Name)
			actualStateMutatingWebhookConfigurationList.Items[0].ObjectMeta.Annotations = zitiwebhook.GetDefaultAnnotations()
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to MutatingWebhookConfiguration", "Name", actualStateMutatingWebhookConfigurationList.Items[0].Name)
			if err := r.Patch(ctx, &actualStateMutatingWebhookConfigurationList.Items[0], client.MergeFrom(existingWebhookForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch MutatingWebhookConfiguration")
				log.Error(err, "Failed to patch MutatingWebhookConfiguration")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched MutatingWebhookConfiguration")
		} else {
			log.V(4).Info("MutatingWebhookConfiguration is up to date", "Name", actualStateMutatingWebhookConfigurationList.Items[0].Name)
		}
	} else {
		log.Error(err, "Failed to get MutatingWebhookConfiguration")
		return ctrl.Result{}, err
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
	} else if err == nil {
		existingWebhookDeploymentForPatch := actualStateWebhookDeployment.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateWebhookDeployment.ObjectMeta.Labels, desiredStateWebhookDeployment.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "Deployment.Name", actualStateWebhookDeployment.Name)
			actualStateWebhookDeployment.ObjectMeta.Labels = zitiwebhook.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateWebhookDeployment, zitiwebhook) {
			log.V(4).Info("Ownership missing, preparing patch", "Deployment.Name", actualStateWebhookDeployment.Name)
			if err := controllerutil.SetControllerReference(zitiwebhook, actualStateWebhookDeployment, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual Deployment for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		if !reflect.DeepEqual(actualStateWebhookDeployment.Spec, desiredStateWebhookDeployment.Spec) {
			log.V(4).Info("Spec differs, preparing patch", "Deployment.Name", actualStateWebhookDeployment.Name)
			actualStateWebhookDeployment.Spec = desiredStateWebhookDeployment.Spec
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to Deployment", "Deployment.Name", actualStateWebhookDeployment.Name)
			if err := r.Patch(ctx, actualStateWebhookDeployment, client.MergeFrom(existingWebhookDeploymentForPatch)); err != nil {
				r.Recorder.Event(zitiwebhook, corev1.EventTypeWarning, "Failed", "Failed to patch Deployment")
				log.Error(err, "Failed to patch Deployment")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitiwebhook, corev1.EventTypeNormal, "Patched", "Patched Deployment")
		} else {
			log.V(4).Info("Deployment is up to date", "Deployment.Name", actualStateWebhookDeployment.Name)
		}
	} else {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Re-fetch the ZitiWebhook object before updating the status
	if err := r.Get(ctx, req.NamespacedName, zitiwebhook); err == nil {
		// Create a copy *before* modifying the status
		existing := zitiwebhook.DeepCopy()
		// Update the status
		zitiwebhook.Status.DeploymentConditions = utils.ConvertDeploymentConditions(actualStateWebhookDeployment.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.DeploymentConditions)
		zitiwebhook.Status.IssuerConditions = utils.ConvertIssuerConditions(actualStateIssuer.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.IssuerConditions)
		zitiwebhook.Status.CertificateConditions = utils.ConvertCertificateConditions(actualStateWebhookCert.Status.Conditions)
		log.V(5).Info("ZitiWebhook Conditions", "Conditions", zitiwebhook.Status.CertificateConditions)
		zitiwebhook.Status.Replicas = actualStateWebhookDeployment.Status.ReadyReplicas
		log.V(5).Info("ZitiWebhook Ready Replicas", "Ready Replicas", zitiwebhook.Status.Replicas)

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
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &rbacv1.ClusterRole{}, "metadata.name", func(rawObj client.Object) []string {
		cr := rawObj.(*rbacv1.ClusterRole)
		return []string{cr.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &rbacv1.ClusterRoleBinding{}, "metadata.name", func(rawObj client.Object) []string {
		crb := rawObj.(*rbacv1.ClusterRoleBinding)
		return []string{crb.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &admissionregistrationv1.MutatingWebhookConfiguration{}, "metadata.name", func(rawObj client.Object) []string {
		mwc := rawObj.(*admissionregistrationv1.MutatingWebhookConfiguration)
		return []string{mwc.Name}
	}); err != nil {
		return err
	}
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
			Labels:    zitiwebhook.GetDefaultLabels(),
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
			Labels:    zitiwebhook.GetDefaultLabels(),
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
			Labels:    zitiwebhook.GetDefaultLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "https",
					Protocol: corev1.ProtocolTCP,
					Port:     *zitiwebhook.Spec.MutatingWebhookSpec[0].ClientConfig.Service.Port,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: zitiwebhook.Spec.DeploymentSpec.Port,
					},
				},
			},
			InternalTrafficPolicy: &cluster,
			IPFamilies:            []corev1.IPFamily{corev1.IPv4Protocol},
			IPFamilyPolicy:        &singleStack,
			Selector:              utils.FilterLabels(zitiwebhook.GetDefaultLabels()),
			SessionAffinity:       corev1.ServiceAffinityNone,
			Type:                  corev1.ServiceTypeClusterIP,
		},
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateServiceAccount(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *corev1.ServiceAccount {
	_ = log.FromContext(ctx)
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-service-account",
			Namespace: zitiwebhook.Namespace,
			Labels:    zitiwebhook.GetDefaultLabels(),
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
			Name:   zitiwebhook.Spec.Name + "-cluster-role",
			Labels: zitiwebhook.GetDefaultLabels(),
		},
		Rules: zitiwebhook.Spec.ClusterRoleSpec.Rules,
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateClusterRoleBinding(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *rbacv1.ClusterRoleBinding {
	_ = log.FromContext(ctx)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   zitiwebhook.Spec.Name + "-cluster-role-binding",
			Labels: zitiwebhook.GetDefaultLabels(),
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
			Name:        zitiwebhook.Spec.Name + "-mutating-webhook-configuration",
			Labels:      zitiwebhook.GetDefaultLabels(),
			Annotations: zitiwebhook.GetDefaultAnnotations(),
		},
		Webhooks: zitiwebhook.Spec.MutatingWebhookSpec,
	}
}

func (r *ZitiWebhookReconciler) getDesiredStateDeploymentConfiguration(ctx context.Context, zitiwebhook *kubernetesv1alpha1.ZitiWebhook) *appsv1.Deployment {
	_ = log.FromContext(ctx)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitiwebhook.Spec.Name + "-deployment",
			Namespace: zitiwebhook.Namespace,
			Labels:    zitiwebhook.GetDefaultLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zitiwebhook.Spec.DeploymentSpec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: utils.FilterLabels(zitiwebhook.GetDefaultLabels()),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: zitiwebhook.GetDefaultLabels(),
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
