package webhook

import (
	"context"
	"fmt"

	"github.com/openziti/edge-api/rest_management_api_client"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ZitiTunnelHandler handles admission requests for Ziti tunnels.
type zitiHandler struct {
	KC     ClusterClient
	ZC     ZitiClient
	Config *ZitiConfig
}

type clusterClient struct {
	Client *kubernetes.Clientset
}

// ClusterClient defines the interface for Kubernetes client operations.
type ClusterClient interface {
	CheckNamespace(ctx context.Context, name string, opts metav1.ListOptions) (bool, error)
	GetDNSService(ctx context.Context, namespace, name string) (*corev1.Service, error)
	GetLabels(ctx context.Context, namespace string) (map[string]string, error)
}

type zitiClient struct {
	Client *rest_management_api_client.ZitiEdgeManagement
}

// ZitiClient defines the interface for Ziti client operations.
type ZitiClient interface {
	GetIdentityToken(appName, sidecarPrefix, uid string, roles []string) (string, string, error)
	FindIdentity(name string) (string, bool, error)
	DeleteIdentity(id string) error
	PatchIdentity(id string, roles []string) (interface{}, error)
}

// Ziti Config holds configuration for the sidecar/router container.
type ZitiConfig struct {
	Image           string
	ImageVersion    string
	VolumeMountName string
	Prefix          string
	RoleKey         string
	LabelKey        string
	labelDelValue   string
	labelCrValue    string
}

type ZitiHandler interface {
	HandleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

// HandleAdmissionRequest handles the admission request for old pod types, i.e. tunnel, router, etc.
func (zh *zitiHandler) HandleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := &corev1.Pod{}
	oldPod := &corev1.Pod{}

	// Decode the Pod and OldPod objects.
	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, pod); err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, oldPod); err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	klog.Infof("Admission Request UID: %s", ar.Request.UID)
	klog.Infof("Admission Request Operation: %s", ar.Request.Operation)
	klog.Infof("Ziti Config LableKey: %s", zh.Config.LabelKey)

	// Check namespace labels for delete action.
	deleteLabelExists, err := zh.KC.CheckNamespace(
		context.Background(),
		pod.Namespace,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", zh.Config.LabelKey, zh.Config.labelDelValue),
		},
	)
	if err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, err)
	}

	// Handle admission operations.
	switch ar.Request.Operation {
	case "CREATE":
		if !deleteLabelExists {
			klog.Infof("Creating: delete action %v", deleteLabelExists)
			// return zh.handleCreate(pod, ar.Request.UID, zitiConfig, reviewResponse)
		}
	case "DELETE":
		klog.Infof("Deleting: delete action %v", deleteLabelExists)
		// return zh.handleDelete(oldPod, zitiConfig, reviewResponse)
	case "UPDATE":
		if !deleteLabelExists {
			klog.Infof("Updating: delete action %v", deleteLabelExists)
			// return zh.handleUpdate(pod, oldPod, zitiConfig, reviewResponse)
		}
	}

	return successResponse(reviewResponse)
}

// // handleCreate handles the CREATE operation.
// func (h *ZitiTunnelHandler) handleCreate(pod corev1.Pod, uid string, zitiConfig ZitiConfig, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {
// 	roles := getRoles(pod.Annotations, zh.SidecarConfig.RoleKey, pod.Labels["app"])

// 	identityToken, sidecarName, err := zh.ZitiClient.GetIdentityToken(pod.Labels["app"], zh.SidecarConfig.Prefix, uid, roles)
// 	if err != nil {
// 		return failureResponse(response, err)
// 	}

// 	// Add sidecar container to the pod.
// 	pod.Spec.Containers = append(pod.Spec.Containers, zh.createSidecarContainer(sidecarName, identityToken))
// 	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
// 		Name: zh.SidecarConfig.VolumeMountName,
// 		VolumeSource: corev1.VolumeSource{
// 			EmptyDir: &corev1.EmptyDirVolumeSource{},
// 		},
// 	})

// 	// Update DNS configuration.
// 	pod.Spec.DNSConfig = &corev1.PodDNSConfig{
// 		Nameservers: []string{"127.0.0.1", getClusterDNSIP(zh.KubeClient)},
// 		Searches:    []string{"cluster.local", fmt.Sprintf("%s.svc", pod.Namespace)},
// 	}
// 	pod.Spec.DNSPolicy = "None"

// 	// Create JSON patch for the pod.
// 	patch, err := createPatch(pod)
// 	if err != nil {
// 		klog.Error(err)
// 		return failureResponse(response, err)
// 	}

// 	response.Patch = patch
// 	pt := admissionv1.PatchTypeJSONPatch
// 	response.PatchType = &pt
// 	return &response
// }

// // handleDelete handles the DELETE operation.
// func (h *ZitiTunnelHandler) handleDelete(pod corev1.Pod, zitiConfig ZitiConfig, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {
// 	sidecarName := fmt.Sprintf("%s-%s", pod.Labels["app"], zh.SidecarConfig.Prefix)
// 	if hasContainer(pod.Spec.Containers, sidecarName) {
// 		zId, ok, err := zh.ZitiClient.FindIdentity(sidecarName)
// 		if err != nil {
// 			return failureResponse(response, err)
// 		}
// 		if ok {
// 			if err := zh.ZitiClient.DeleteIdentity(zId); err != nil {
// 				return failureResponse(response, err)
// 			}
// 		}
// 	}
// 	return successResponse(response)
// }

// // handleUpdate handles the UPDATE operation.
// func (h *ZitiTunnelHandler) handleUpdate(pod, oldPod corev1.Pod, zitiConfig ZitiConfig, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {
// 	sidecarName := fmt.Sprintf("%s-%s", pod.Labels["app"], zh.SidecarConfig.Prefix)
// 	if hasContainer(pod.Spec.Containers, sidecarName) {
// 		newRoles := getRoles(pod.Annotations, zh.SidecarConfig.RoleKey, pod.Labels["app"])
// 		oldRoles := getRoles(oldPod.Annotations, zh.SidecarConfig.RoleKey, oldPod.Labels["app"])

// 		if !reflect.DeepEqual(newRoles, oldRoles) {
// 			zId, ok, err := zh.ZitiClient.FindIdentity(sidecarName)
// 			if err != nil {
// 				return failureResponse(response, err)
// 			}
// 			if ok {
// 				if _, err := zh.ZitiClient.PatchIdentity(zId, newRoles); err != nil {
// 					return failureResponse(response, err)
// 				}
// 			}
// 		}
// 	}
// 	return successResponse(response)
// }

// // Helper functions (e.g., decodeObject, getRoles, createPatch, etc.) go here.

func (c *clusterClient) CheckNamespace(ctx context.Context, name string, opts metav1.ListOptions) (bool, error) {
	namespaces, err := c.Client.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		return false, err
	}
	for _, ns := range namespaces.Items {
		if ns.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (c *clusterClient) GetDNSService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	return c.Client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *clusterClient) GetLabels(ctx context.Context, namespace string) (map[string]string, error) {
	ns, err := c.Client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ns.Labels, nil
}

func (z *zitiClient) GetIdentityToken(appName, sidecarPrefix, uid string, roles []string) (string, string, error) {
	return "", "", nil
}

func (z *zitiClient) FindIdentity(name string) (string, bool, error) {
	return "", false, nil
}

func (z *zitiClient) DeleteIdentity(id string) error {
	return nil
}

func (z *zitiClient) PatchIdentity(id string, roles []string) (interface{}, error) {
	return nil, nil
}

// NewZitiTunnelHandler creates a new ZitiTunnelHandler.
func NewZitiHandler(config *ZitiConfig, zc *ZitiClient, kc *ClusterClient) *zitiHandler {
	return &zitiHandler{
		KC:     &clusterClient{},
		ZC:     &zitiClient{},
		Config: config,
	}
}
