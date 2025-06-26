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
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ZitiRouterReconciler reconciles a ZitiRouter object
type ZitiRouterReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ZitiControllerChan   chan *kubernetesv1alpha1.ZitiController
	CachedZitiController *kubernetesv1alpha1.ZitiController
}

// const (
// 	orphanPvcAnnotationPrefix    = "zitirouter.kubernetes.openziti.io/"
// 	orphanPvcTimestampAnnotation = orphanPvcAnnotationPrefix + "orphaned-since"
// )

// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters/finalizers,verbs=update
// +kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiRouter object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiRouterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("ZitiRouter Reconciliation started")

	zitirouter := &kubernetesv1alpha1.ZitiRouter{}
	if err := r.Get(ctx, req.NamespacedName, zitirouter); err != nil && apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	// Merge defaults and ziticontroller specs if changes are detected
	log.V(5).Info("zitirouter Actual", "Name", zitirouter.Name, "Specs", zitirouter.Spec)
	defaultSpecs := zitirouter.GetDefaults()
	log.V(5).Info("zitirouter Default", "Name", zitirouter.Name, "Specs", defaultSpecs)
	err, ok := utils.MergeSpecs(ctx, &zitirouter.Spec, defaultSpecs)
	if err == nil && ok {
		select {
		case ziticontroller := <-r.ZitiControllerChan:
			log.V(5).Info("ZitiController Spec", "Name", ziticontroller.Spec.Name, "ZitiController.Spec", ziticontroller.Spec)
			r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Update", "Using ZitiController from channel")
			r.CachedZitiController = ziticontroller
			zitirouter.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
			zitirouter.Spec.ZitiCtrlMgmtApi = r.CachedZitiController.Spec.ZitiCtrlMgmtApi
			if zitirouter.Spec.ZitiCtrlMgmtApi == "" {
				zitirouter.Spec.ZitiCtrlMgmtApi, _ = utils.GetUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
				zitirouter.Spec.ZitiCtrlMgmtApi = zitirouter.Spec.ZitiCtrlMgmtApi + "/edge/management/v1"
				log.V(5).Info("ZitiController URL", "ZitiCtrlMgmtApi", zitirouter.Spec.ZitiCtrlMgmtApi)
			}
			if zitirouter.Spec.Config.Ctrl.Endpoint == "" {
				host, port, _ := utils.GetHostAndPort(zitirouter.Spec.ZitiCtrlMgmtApi)
				zitirouter.Spec.Config.Ctrl.Endpoint = "tls:" + host + ":" + port
			}
		default:
			if r.CachedZitiController != nil {
				log.V(5).Info("Cached ZitiController Spec", "Name", r.CachedZitiController.Spec.Name, "ZitiController.Spec", r.CachedZitiController.Spec)
				zitirouter.Spec.ZitiControllerName = r.CachedZitiController.Spec.Name
				zitirouter.Spec.ZitiCtrlMgmtApi = r.CachedZitiController.Spec.ZitiCtrlMgmtApi
				if zitirouter.Spec.ZitiCtrlMgmtApi == "" {
					zitirouter.Spec.ZitiCtrlMgmtApi, _ = utils.GetUrlFromJwt(r.CachedZitiController.Spec.AdminJwt)
					zitirouter.Spec.ZitiCtrlMgmtApi = zitirouter.Spec.ZitiCtrlMgmtApi + "/edge/management/v1"
					log.V(5).Info("ZitiController URL", "ZitiCtrlMgmtApi", zitirouter.Spec.ZitiCtrlMgmtApi)
				}
				if zitirouter.Spec.Config.Ctrl.Endpoint == "" {
					host, port, _ := utils.GetHostAndPort(zitirouter.Spec.ZitiCtrlMgmtApi)
					zitirouter.Spec.Config.Ctrl.Endpoint = "tls:" + host + ":" + port
				}
			} else {
				log.V(5).Info("No ZitiController Spec available")
				r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "No ZitiController Spec available")
			}
		}
		if err := r.Update(ctx, zitirouter); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Merged", "Merged default specs to zitirouter")
		log.V(2).Info("zitirouter Merged", "Name", zitirouter.Name, "Specs", zitirouter.Spec)
	} else if err != nil {
		log.V(5).Info("zitirouter Spec merge failed", "Name", zitirouter.Name, "Error", err)
		log.V(5).Info("zitirouter Spec merge failed", "Name", zitirouter.Name, "Ok is", ok)
		r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "zitirouter Spec merge failed")
		return ctrl.Result{}, err
	}

	actualStateConfigMap := &corev1.ConfigMap{}
	desiredStateConfigMap := r.getDesiredStateConfigMap(ctx, zitirouter)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitirouter.Namespace,
		Name:      zitirouter.Spec.Name + "-config",
	}, actualStateConfigMap); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new ConfigMap", "ConfigMap.Namespace", desiredStateConfigMap.Namespace, "ConfigMap.Name", desiredStateConfigMap.Name)
		log.V(5).Info("Creating a new ConfigMap", "ConfigMap.Namespace", desiredStateConfigMap.Namespace, "ConfigMap.Spec", desiredStateConfigMap.Data)
		if err := controllerutil.SetControllerReference(zitirouter, desiredStateConfigMap, r.Scheme); err != nil {
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to set controller reference")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateConfigMap); err != nil {
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Created", "Created a new ConfigMap")
	} else if err == nil {
		existingConfigMapForPatch := actualStateConfigMap.DeepCopy()
		needsPatch := false
		if !reflect.DeepEqual(actualStateConfigMap.ObjectMeta.Labels, desiredStateConfigMap.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "ConfigMap.Name", actualStateConfigMap.Name)
			actualStateConfigMap.ObjectMeta.Labels = zitirouter.GetDefaultLabels()
			needsPatch = true
		}
		if !metav1.IsControlledBy(actualStateConfigMap, zitirouter) {
			log.V(4).Info("Ownership missing, preparing patch", "ConfigMap.Name", actualStateConfigMap.Name)
			if err := controllerutil.SetControllerReference(zitirouter, actualStateConfigMap, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual ConfigMap for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}
		if !reflect.DeepEqual(actualStateConfigMap.Data, desiredStateConfigMap.Data) {
			log.V(4).Info("Data differs, preparing patch", "ConfigMap.Name", actualStateConfigMap.Name)
			log.V(4).Info("Data differs, preparing patch", "ConfigMap.Data", actualStateConfigMap.Data)
			log.V(4).Info("Data differs, preparing patch", "Desired.Data", desiredStateConfigMap.Data)
			actualStateConfigMap.Data = desiredStateConfigMap.Data
			needsPatch = true
		}
		if needsPatch {
			log.V(4).Info("Applying patch to ConfigMap", "ConfigMap.Name", actualStateConfigMap.Name)
			if err := r.Patch(ctx, actualStateConfigMap, client.MergeFrom(existingConfigMapForPatch)); err != nil {
				r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to patch ConfigMap")
				log.Error(err, "Failed to patch ConfigMap")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Patched", "Patched ConfigMap")
		} else {
			log.V(4).Info("ConfigMap is up to date", "ConfigMap.Name", actualStateConfigMap.Name)
		}
	} else {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	actualStateService := &corev1.Service{}
	desiredStateService := r.getDesiredStateService(ctx, zitirouter)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitirouter.Namespace,
		Name:      zitirouter.Spec.Name + "-service",
	}, actualStateService); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new Service", "Service.Namespace", desiredStateService.Namespace, "Service.Name", desiredStateService.Name)
		log.V(5).Info("Creating a new Service", "Service.Namespace", desiredStateService.Namespace, "Service.Spec", desiredStateService.Spec)
		if err := controllerutil.SetControllerReference(zitirouter, desiredStateService, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateService); err != nil {
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to create Service")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Created", "Created a new Service")
	} else if err == nil {
		existingServicetForPatch := actualStateService.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateService.ObjectMeta.Labels, desiredStateService.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "Service.Name", actualStateService.Name)
			actualStateService.ObjectMeta.Labels = zitirouter.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateService, zitirouter) {
			log.V(4).Info("Ownership missing, preparing patch", "Service.Name", actualStateService.Name)
			if err := controllerutil.SetControllerReference(zitirouter, actualStateService, r.Scheme); err != nil {
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
				r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to patch Service")
				log.Error(err, "Failed to patch Service")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Patched", "Patched Service")
		} else {
			log.V(4).Info("Service is up to date", "Service.Name", actualStateService.Name)
		}
	} else {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	actualStateRouterStatefulSet := &appsv1.StatefulSet{}
	desiredStateRouterStatefulSet := r.getDesiredStateStatefulSetConfiguration(ctx, zitirouter)
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: zitirouter.Namespace,
		Name:      zitirouter.Spec.Name + "-statefulset",
	}, actualStateRouterStatefulSet); err != nil && apierrors.IsNotFound(err) {
		log.V(4).Info("Creating a new StatefulSet", "StatefulSet.Namespace", desiredStateRouterStatefulSet.Namespace, "StatefulSet.Name", desiredStateRouterStatefulSet.Name)
		log.V(5).Info("Creating a new StatefulSet", "StatefulSet.Namespace", desiredStateRouterStatefulSet.Namespace, "StatefulSet.Spec", desiredStateRouterStatefulSet.Spec)
		if err := ctrl.SetControllerReference(zitirouter, desiredStateRouterStatefulSet, r.Scheme); err != nil {
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to set controller reference")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredStateRouterStatefulSet); err != nil {
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to create StatefulSet")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Created", "Created a new StatefulSet")
	} else if err == nil {
		existingWebhookStatefulSetForPatch := actualStateRouterStatefulSet.DeepCopy()
		needsPatch := false

		if !reflect.DeepEqual(actualStateRouterStatefulSet.ObjectMeta.Labels, desiredStateRouterStatefulSet.ObjectMeta.Labels) {
			log.V(4).Info("Labels differ, preparing patch", "StatefulSet.Name", actualStateRouterStatefulSet.Name)
			actualStateRouterStatefulSet.ObjectMeta.Labels = zitirouter.GetDefaultLabels()
			needsPatch = true
		}

		if !metav1.IsControlledBy(actualStateRouterStatefulSet, zitirouter) {
			log.V(4).Info("Ownership missing, preparing patch", "StatefulSet.Name", actualStateRouterStatefulSet.Name)
			if err := controllerutil.SetControllerReference(zitirouter, actualStateRouterStatefulSet, r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on actual StatefulSet for patch")
				return ctrl.Result{}, err
			}
			needsPatch = true
		}

		// sync PVC Status otherwise, it may always be differnet
		desiredStateRouterStatefulSet.Spec.VolumeClaimTemplates[0].Status = actualStateRouterStatefulSet.Spec.VolumeClaimTemplates[0].Status
		if !utils.DeepEqualExcludingFields(actualStateRouterStatefulSet.Spec, desiredStateRouterStatefulSet.Spec, "Template") {
			log.V(4).Info("Spec w/o Template differs, preparing patch", "Current.Spec w/o Template", actualStateRouterStatefulSet.Spec)
			log.V(4).Info("Spec w/o Template differs, preparing patch", "Desired.Spec w/o Template", desiredStateRouterStatefulSet.Spec)
			r.copyStatefulSetSpecExcludingTemplate(&actualStateRouterStatefulSet.Spec, &desiredStateRouterStatefulSet.Spec)
			needsPatch = true
		}

		if !utils.DeepEqualExcludingFields(actualStateRouterStatefulSet.Spec.Template.Spec, desiredStateRouterStatefulSet.Spec.Template.Spec) {
			log.V(4).Info("PodSpec differs, preparing patch", "Current.Template.Spec", actualStateRouterStatefulSet.Spec.Template.Spec)
			log.V(4).Info("PodSpec differs, preparing patch", "Desired.Template.Spec", desiredStateRouterStatefulSet.Spec.Template.Spec)
			actualStateRouterStatefulSet.Spec.Template.Spec = desiredStateRouterStatefulSet.Spec.Template.Spec
			needsPatch = true
		}

		if needsPatch {
			log.V(4).Info("Applying patch to StatefulSet", "StatefulSet.Name", actualStateRouterStatefulSet.Name)
			if err := r.Patch(ctx, actualStateRouterStatefulSet, client.MergeFrom(existingWebhookStatefulSetForPatch)); err != nil {
				r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to patch StatefulSet")
				log.Error(err, "Failed to patch StatefulSet")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(zitirouter, corev1.EventTypeNormal, "Patched", "Patched StatefulSet")
		} else {
			log.V(4).Info("StatefulSet is up to date", "StatefulSet.Name", actualStateRouterStatefulSet.Name)
		}

	} else {
		log.Error(err, "Failed to get StatefulSet")
		return ctrl.Result{}, err
	}

	// Re-fetch the ZitiRouter object before updating the status
	if err := r.Get(ctx, req.NamespacedName, zitirouter); err == nil {
		// Create a copy *before* modifying the status
		existing := zitirouter.DeepCopy()
		// Update the status
		// zitirouter.Status.DeploymentConditions = utils.ConvertDeploymentConditions(actualStaterouterDeployment.Status.Conditions)
		// log.V(5).Info("Zitirouter Conditions", "Conditions", zitirouter.Status.DeploymentConditions)
		zitirouter.Status.Replicas = actualStateRouterStatefulSet.Status.ReadyReplicas
		log.V(5).Info("Zitirouter Ready Replicas", "Ready Replicas", zitirouter.Status.Replicas)

		// Attempt to patch the status
		if err := r.Status().Patch(ctx, zitirouter, client.MergeFrom(existing)); err != nil {
			log.Error(err, "Failed to patch Zitirouter status")
			r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to update Zitirouter status")
			return ctrl.Result{}, err
		}
	} else {
		r.Recorder.Event(zitirouter, corev1.EventTypeWarning, "Failed", "Failed to get Zitirouter")
		return ctrl.Result{}, err
	}

	log.V(2).Info("ZitiRouter Reconciliation complete", "name", req.Name)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiRouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("zitirouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubernetesv1alpha1.ZitiRouter{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *ZitiRouterReconciler) getDesiredStateConfigMap(ctx context.Context, zitirouter *kubernetesv1alpha1.ZitiRouter) *corev1.ConfigMap {
	_ = log.FromContext(ctx)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.Name + "-config",
			Namespace: zitirouter.Namespace,
			Labels:    zitirouter.GetDefaultLabels(),
		},
		Data: zitirouter.GetDefaultConfigMapData(),
	}
}

func (r *ZitiRouterReconciler) getDesiredStateService(ctx context.Context, zitirouter *kubernetesv1alpha1.ZitiRouter) *corev1.Service {
	_ = log.FromContext(ctx)
	cluster := corev1.ServiceInternalTrafficPolicyCluster
	singleStack := corev1.IPFamilyPolicySingleStack
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.Name + "-service",
			Namespace: zitirouter.Namespace,
			Labels:    zitirouter.GetDefaultLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     zitirouter.Spec.Deployment.Container.Ports[0].Name,
					Protocol: zitirouter.Spec.Deployment.Container.Ports[0].Protocol,
					Port:     zitirouter.GetDefaultServicePort(),
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: zitirouter.Spec.Deployment.Container.Ports[0].ContainerPort,
					},
				},
			},
			InternalTrafficPolicy: &cluster,
			IPFamilies:            []corev1.IPFamily{corev1.IPv4Protocol},
			IPFamilyPolicy:        &singleStack,
			Selector:              utils.FilterLabels(zitirouter.GetDefaultLabels()),
			SessionAffinity:       corev1.ServiceAffinityNone,
			Type:                  corev1.ServiceTypeClusterIP,
		},
	}
}

func (r *ZitiRouterReconciler) getDesiredStateStatefulSetConfiguration(ctx context.Context, zitirouter *kubernetesv1alpha1.ZitiRouter) *appsv1.StatefulSet {
	_ = log.FromContext(ctx)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.Name + "-statefulset",
			Namespace: zitirouter.Namespace,
			Labels:    zitirouter.Spec.Deployment.Labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: zitirouter.Spec.Deployment.Replicas,
			Selector: zitirouter.Spec.Deployment.Selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      zitirouter.Spec.Deployment.Labels,
					Annotations: zitirouter.Spec.Deployment.Annotations,
				},
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{zitirouter.Spec.Deployment.Container},
					HostNetwork:                   zitirouter.Spec.Deployment.HostNetwork,
					DNSConfig:                     zitirouter.Spec.Deployment.DNSConfig,
					DNSPolicy:                     zitirouter.Spec.Deployment.DNSPolicy,
					SchedulerName:                 zitirouter.Spec.Deployment.SchedulerName,
					RestartPolicy:                 zitirouter.Spec.Deployment.RestartPolicy,
					SecurityContext:               zitirouter.Spec.Deployment.SecurityContext,
					TerminationGracePeriodSeconds: zitirouter.Spec.Deployment.TerminationGracePeriodSeconds,
					Volumes:                       zitirouter.Spec.Deployment.Volumes,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      zitirouter.Spec.Name,
						Namespace: zitirouter.Namespace,
						Labels:    zitirouter.Spec.Deployment.Labels,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("50Mi"),
							},
						},
						StorageClassName: zitirouter.Spec.Deployment.StorageClassName,
						VolumeMode:       zitirouter.Spec.Deployment.VolumeMode,
					},
				},
			},
			ServiceName:                          zitirouter.Spec.Name + "-service",
			PodManagementPolicy:                  appsv1.ParallelPodManagement,
			UpdateStrategy:                       zitirouter.Spec.Deployment.UpdateStrategy,
			RevisionHistoryLimit:                 zitirouter.Spec.Deployment.RevisionHistoryLimit,
			MinReadySeconds:                      zitirouter.Spec.Deployment.MinReadySeconds,
			PersistentVolumeClaimRetentionPolicy: zitirouter.Spec.Deployment.PersistentVolumeClaimRetentionPolicy,
			Ordinals:                             zitirouter.Spec.Deployment.Ordinals,
		},
	}
}

func (r *ZitiRouterReconciler) copyStatefulSetSpecExcludingTemplate(dest, src *appsv1.StatefulSetSpec) {
	dest.Replicas = src.Replicas
	dest.Selector = src.Selector
	dest.ServiceName = src.ServiceName
	dest.PodManagementPolicy = src.PodManagementPolicy
	dest.UpdateStrategy = src.UpdateStrategy
	dest.RevisionHistoryLimit = src.RevisionHistoryLimit
	dest.MinReadySeconds = src.MinReadySeconds
	dest.VolumeClaimTemplates = src.VolumeClaimTemplates
	dest.PersistentVolumeClaimRetentionPolicy = src.PersistentVolumeClaimRetentionPolicy
	dest.Ordinals = src.Ordinals
	// Template field is intentionally excluded
}
