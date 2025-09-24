package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	zitiedge "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"
	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_management_api_client/edge_router"
	rest_model_edge "github.com/openziti/edge-api/rest_model"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	rootUser     int64 = 0
	isNotTrue    bool  = false
	isPrivileged bool  = false
	jsonPatch    []JsonPatchEntry
	pvc          *corev1.PersistentVolumeClaim
)

const (
	// Container Default Limits
	defaultRequestsResourceCPU    = "50m"
	defaultRequestsResourceMemory = "64Mi"
	defaultLimitsResourceCPU      = "100m"
	defaultLimitsResourceMemory   = "128Mi"

	// Annotation key for explicitly setting identity name
	annotationIdentityName = "identity.openziti.io/name"

	// Label keys in order of precedence
	labelApp          = "app"
	labelAppName      = "app.kubernetes.io/name"
	labelAppInstance  = "app.kubernetes.io/instance"
	labelAppComponent = "app.kubernetes.io/component"

	// warnEmptyJWT            = "pod created without ziti enrollment token"
	// warnIdentityNameExtract = "ziti identity name not found in pod annotations"
	// warnIdentityListFailed  = "ziti identity list operation failed in ziti edge"

	zitiTypeRouter zitiType = "router"
	zitiTypeTunnel zitiType = "tunnel"
)

type zitiType string

type clusterClient struct {
	client *kubernetes.Clientset
}

type clusterClientIntf interface {
	getClusterService(ctx context.Context, namespace string, name string, opts metav1.GetOptions) (*corev1.Service, error)
	findNamespaceByOption(ctx context.Context, name string, opts metav1.ListOptions) (bool, error)
	getPvcByOption(ctx context.Context, namespace string, name string, opts metav1.GetOptions) (*corev1.PersistentVolumeClaim, error)
	deletePvc(ctx context.Context, namespace string, name string) error
}

type zitiClient struct {
	client *rest_management_api_client.ZitiEdgeManagement
}

type zitiClientIntf interface {
	createIdentity(ctx context.Context, name string, roleKey string, podMeta *metav1.ObjectMeta) (string, error)
	deleteIdentity(ctx context.Context, id string) error
	deleteZitiRouter(ctx context.Context, name string) error
	getIdentityToken(ctx context.Context, name string, id string) (string, error)
	getZitiRouterToken(ctx context.Context, name string) (string, error)
	findIdentityId(ctx context.Context, name string) (string, error)
	patchIdentityRoleAttributes(ctx context.Context, id string, key string, newPod *corev1.Pod, oldPod *corev1.Pod) error
	updateZitiRouter(ctx context.Context, name string, options *rest_model_edge.EdgeRouterCreate) (*edge_router.CreateEdgeRouterCreated, error)
}

type zitiConfig struct {
	Image           string
	ImageVersion    string
	ImagePullPolicy string
	VolumeMountName string
	IdentityDir     string
	Prefix          string
	RoleKey         string
	LabelKey        string
	LabelDelValue   string
	LabelCrValue    string
	ResolverIp      string
	DnsUpstreamEnabled bool
	Unanswerable       string
	ZitiType        zitiType
	AnnotationKey   string
	RouterConfig    routerConfig
}

type routerConfig struct {
	Cost              int64
	Disabled          bool
	IsTunnelerEnabled bool
	RoleAttributes    rest_model_edge.Attributes
}

type zitiHandler struct {
	KC     clusterClientIntf
	ZC     zitiClientIntf
	Config *zitiConfig
}

type ZitiHandler interface {
	handleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
	handleTunnelCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse
	handleRouterCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse
	handleDelete(ctx context.Context, pod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse
	handleUpdate(ctx context.Context, pod *corev1.Pod, oldPod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse
}

// handleAdmissionRequest handles Kubernetes admission requests for pod operations.
// It processes "CREATE", "DELETE", and "UPDATE" operations to manage Ziti identities
// and associated Kubernetes resources based on pod annotations and labels.
//
// Args:
//
//	ar: AdmissionReview object containing the admission request details.
//
// Returns:
//
//	A pointer to the AdmissionResponse indicating success or failure
//	of the admission request processing.
func (zh *zitiHandler) handleAdmissionRequest(ctx context.Context, ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := &corev1.Pod{}
	oldPod := &corev1.Pod{}

	klog.V(5).Infof("Object: %s", ar.Request.Object.Raw)
	klog.V(5).Infof("OldObject: %s", ar.Request.OldObject.Raw)

	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, pod); err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, fmt.Errorf("failed to decode pod object: %v", err))
	}
	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, oldPod); err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, fmt.Errorf("failed to decode old pod object: %v", err))
	}

	klog.Infof("%s operation admission request UID: %s", ar.Request.Operation, ar.Request.UID)

	// create a context to pass to subsequent functions allowing cancellations to propagate

	deleteLabelFound, err := zh.KC.findNamespaceByOption(
		ctx,
		pod.Namespace,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", zh.Config.LabelKey, zh.Config.LabelDelValue),
		},
	)
	if err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, err)
	}

	// Handle admission operations.
	switch ar.Request.Operation {

	case "CREATE":

		klog.V(4).Infof("Starting webhook operation: %s", ar.Request.Operation)
		klog.V(4).Infof("Updating: delete action %v", deleteLabelFound)

		if !deleteLabelFound {

			switch zh.Config.ZitiType {

			case zitiTypeTunnel:

				return zh.handleTunnelCreate(
					ctx,
					&pod.ObjectMeta,
					ar.Request.UID,
					reviewResponse,
				)

			case zitiTypeRouter:

				return zh.handleRouterCreate(
					ctx,
					pod,
					ar.Request.UID,
					reviewResponse,
				)

			default:

				err := fmt.Errorf("ziti type %s not supported", zh.Config.ZitiType)
				return failureResponse(reviewResponse, err)
			}

		}
	case "DELETE":

		klog.V(4).Infof("Starting webhook operation: %s", ar.Request.Operation)
		klog.V(4).Infof("Updating: delete action %v", deleteLabelFound)

		return zh.handleDelete(
			ctx,
			oldPod,
			reviewResponse,
		)
	case "UPDATE":

		klog.V(4).Infof("Starting webhook operation: %s", ar.Request.Operation)
		klog.V(4).Infof("Updating: delete action %v", deleteLabelFound)

		if !deleteLabelFound {

			return zh.handleUpdate(
				ctx,
				pod,
				oldPod,
				reviewResponse,
			)

		}
	}

	reviewResponseJSON, err := json.Marshal(reviewResponse)
	if err != nil {
		klog.Warningf("failed to marshal review response to JSON: %v", err)
	} else {
		klog.V(5).Infof("Review response before passing to admission handler:\n%s", string(reviewResponseJSON))
	}
	return successResponse(reviewResponse)
}

func (zh *zitiHandler) handleTunnelCreate(ctx context.Context, podMeta *metav1.ObjectMeta, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	identityName, err := buildZitiIdentityName(zh.Config.Prefix, podMeta, uid)
	if err != nil {
		return failureResponse(response, err)
	}

	identityId, err := zh.ZC.createIdentity(
		ctx,
		identityName,
		zh.Config.RoleKey,
		podMeta,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	identityToken, err := zh.ZC.getIdentityToken(
		ctx,
		identityName,
		identityId,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	dnsConfig, err := zh.getDnsConfig(ctx, podMeta)
	if err != nil {
		return failureResponse(response, err)
	}

	sidecarArgs := []string{"tproxy"}

	if zh.Config.DnsUpstreamEnabled && zh.Config.ResolverIp != "" {
		sidecarArgs = append(sidecarArgs, "--dnsUpstream", fmt.Sprintf("tcp://%s:53", zh.Config.ResolverIp))
	}

	unanswerable := zh.Config.Unanswerable
	if unanswerable == "" {
		unanswerable = "refused"
	}

	sidecarArgs = append(sidecarArgs, "--dnsUnanswerable", unanswerable)

	jsonPatch = []JsonPatchEntry{

		{
			OP:   "add",
			Path: "/spec/containers/-",
			Value: corev1.Container{
				Name:            identityName,
				Image:           fmt.Sprintf("%s:%s", zh.Config.Image, zh.Config.ImageVersion),
				ImagePullPolicy: corev1.PullPolicy(zh.Config.ImagePullPolicy),
				Args:            sidecarArgs,
				Env: []corev1.EnvVar{
					{
						Name:  "ZITI_ENROLL_TOKEN",
						Value: identityToken,
					},
					{
						Name:  "ZITI_IDENTITY_DIR",
						Value: zh.Config.IdentityDir,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      zh.Config.VolumeMountName,
						MountPath: zh.Config.IdentityDir,
						ReadOnly:  false,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add:  []corev1.Capability{"NET_ADMIN", "NET_BIND_SERVICE"},
						Drop: []corev1.Capability{"ALL"},
					},
					RunAsUser:  &rootUser,
					Privileged: &isPrivileged,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(defaultRequestsResourceCPU),
						corev1.ResourceMemory: resource.MustParse(defaultRequestsResourceMemory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(defaultLimitsResourceCPU),
						corev1.ResourceMemory: resource.MustParse(defaultLimitsResourceMemory),
					},
				},
			},
		},
		{
			OP:   "add",
			Path: "/spec/volumes/-",
			Value: corev1.Volume{
				Name: zh.Config.VolumeMountName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		{
			OP:    "replace",
			Path:  "/spec/dnsPolicy",
			Value: "None",
		},
		{
			OP:    "add",
			Path:  "/spec/dnsConfig",
			Value: dnsConfig,
		},
	}

	if podSecurityOverride {
		jsonPatch = append(jsonPatch, []JsonPatchEntry{
			{
				OP:   "replace",
				Path: "/spec/securityContext",
				Value: &corev1.SecurityContext{
					RunAsNonRoot: &isNotTrue,
				},
			},
		}...)
	}

	if _, ok := podMeta.Annotations[annotationIdentityName]; !ok {
		jsonPatch = append(jsonPatch, []JsonPatchEntry{
			{
				OP:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					annotationIdentityName: identityName,
				},
			},
		}...)
	} else {
		jsonPatch = append(jsonPatch, []JsonPatchEntry{
			{
				OP:   "replace",
				Path: "/metadata/annotations",
				Value: map[string]string{
					annotationIdentityName: identityName,
				},
			},
		}...)
	}

	klog.V(5).Infof("JSON Patch: %v", jsonPatch)
	patchBytes, err := json.Marshal(&jsonPatch)
	if err != nil {
		klog.Error(err)
	}

	response.Patch = patchBytes
	pt := admissionv1.PatchTypeJSONPatch
	response.PatchType = &pt
	return successResponse(response)
}

func (zh *zitiHandler) getDnsConfig(ctx context.Context, podMeta *metav1.ObjectMeta) (*corev1.PodDNSConfig, error) {
	// get cluster dns ip if not already configured
	defaultClusterDnsServiceIP := "10.96.0.10"
	if len(zh.Config.ResolverIp) == 0 {
		service, err := zh.KC.getClusterService(
			ctx,
			"kube-system", "kube-dns",
			metav1.GetOptions{},
		)
		if err != nil {
			klog.Warningf("Failed to look up DNS service: %v", err)
			klog.Warningf("Using default DNS IP: %s", defaultClusterDnsServiceIP)
			zh.Config.ResolverIp = defaultClusterDnsServiceIP
		} else if len(service.Spec.ClusterIP) != 0 {
			zh.Config.ResolverIp = service.Spec.ClusterIP
			klog.V(4).Infof("Using cluster DNS IP: %s", zh.Config.ResolverIp)
		} else {
			zh.Config.ResolverIp = defaultClusterDnsServiceIP
			klog.Warningf("DNS service has no ClusterIP, using default: %s", defaultClusterDnsServiceIP)
		}
	}

	dnsConfig := &corev1.PodDNSConfig{
		Nameservers: []string{
			"127.0.0.1",
			zh.Config.ResolverIp,
		},
		Options: []corev1.PodDNSConfigOption{
			{
				Name:  "ndots",
				Value: &[]string{"5"}[0],
			},
			{
				Name:  "edns0",
				Value: &[]string{"true"}[0],
			},
		},
	}

	if len(searchDomains) > 0 {
		dnsConfig.Searches = searchDomains
		klog.V(4).Infof("Using custom search domains: %v", searchDomains)
	} else {
		// Add namespace-specific search domain
		namespaceDomain := fmt.Sprintf("%s.svc.cluster.local", podMeta.Namespace)
		dnsConfig.Searches = []string{namespaceDomain, "svc.cluster.local", "cluster.local"}
		klog.V(4).Infof("Using default cluster search domains with namespace %s: %v", podMeta.Namespace, dnsConfig.Searches)
	}

	return dnsConfig, nil
}

func (zh *zitiHandler) handleRouterCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	routerName, err := buildZitiIdentityName(zh.Config.Prefix, &pod.ObjectMeta, uid)
	if err != nil {
		return failureResponse(response, err)
	}

	options := &rest_model_edge.EdgeRouterCreate{
		AppData:           nil,
		Cost:              &zh.Config.RouterConfig.Cost,
		Disabled:          &zh.Config.RouterConfig.Disabled,
		IsTunnelerEnabled: zh.Config.RouterConfig.IsTunnelerEnabled,
		Name:              &routerName,
		NoTraversal:       &isNotTrue,
		RoleAttributes:    &zh.Config.RouterConfig.RoleAttributes,
		Tags:              nil,
	}

	_, err = zh.ZC.updateZitiRouter(
		ctx,
		routerName,
		options,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	identityToken, err := zh.ZC.getZitiRouterToken(
		ctx,
		routerName,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	jsonPatch = []JsonPatchEntry{
		{
			OP:    "replace",
			Path:  "/spec/containers/0/env/0/value",
			Value: identityToken,
		},
		{
			OP:    "replace",
			Path:  "/spec/containers/0/env/7/value",
			Value: routerName,
		},
	}

	patchBytes, err := json.Marshal(&jsonPatch)
	if err != nil {
		klog.Error(err)
	}

	response.Patch = patchBytes
	pt := admissionv1.PatchTypeJSONPatch
	response.PatchType = &pt
	return successResponse(response)
}

func (zh *zitiHandler) handleDelete(ctx context.Context, pod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	if zh.Config.ZitiType == zitiTypeRouter {

		if err := zh.ZC.deleteZitiRouter(ctx, pod.Spec.Containers[0].Env[7].Value); err != nil {
			return failureResponse(response, err)
		}

		if pvc, err = zh.KC.getPvcByOption(ctx, pod.Namespace, pod.Labels[labelApp]+"-"+pod.Name, metav1.GetOptions{}); err != nil {
			klog.Errorf("failed to delete PVC for router %s: %v", pod.Spec.Containers[0].Env[7].Value, err)
			return failureResponse(response, fmt.Errorf("failed to delete PVC for router %s: %v", pod.Spec.Containers[0].Env[7].Value, err))
		}

		if pvc != nil && pvc.Name != "" {
			if err := zh.KC.deletePvc(ctx, pod.Namespace, pvc.Name); err != nil {
				klog.Errorf("failed to delete PVC %s for router %s: %v", pvc.Name, pod.Spec.Containers[0].Env[7].Value, err)
				return failureResponse(response, fmt.Errorf("failed to delete PVC %s for router %s: %v", pvc.Name, pod.Spec.Containers[0].Env[7].Value, err))
			}
			klog.V(3).Infof("deleted PVC %s for router %s", pvc.Name, pod.Spec.Containers[0].Env[7].Value)
		}

	} else {

		// Attempt to find a sidecar container identity first.
		if name, containerExists := hasContainer(pod.Spec.Containers, zh.Config.Prefix); containerExists && name != "" {
			klog.V(3).Infof("ziti identity name from container spec is %s", name)
			if err := zh.ZC.deleteIdentity(ctx, name); err != nil {
				return failureResponse(response, err)
			}
			return successResponse(response)
		}

		// Otherwise, look for an annotation-based identity.
		if name, annotationExists := filterMapValueByKey(pod.Annotations, annotationIdentityName); annotationExists && name != "" {
			klog.V(3).Infof("ziti identity name from annotations is %s", name)
			if err := zh.ZC.deleteIdentity(ctx, name); err != nil {
				return failureResponse(response, err)
			}
			return successResponse(response)
		}

		klog.V(3).Infof("no ziti identity name annotation or tunnel sidecar found for pod name '%s'", pod.Name)
	}

	return successResponse(response)
}

func (zh *zitiHandler) handleUpdate(ctx context.Context, pod *corev1.Pod, oldPod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	// Attempt to find a sidecar container identity first.
	if name, containerExists := hasContainer(pod.Spec.Containers, zh.Config.Prefix); containerExists && name != "" {
		klog.V(3).Infof("ziti identity name from container spec is %s", name)
		if err := zh.ZC.patchIdentityRoleAttributes(ctx, name, zh.Config.RoleKey, pod, oldPod); err != nil {
			return failureResponse(response, err)
		}
		return successResponse(response)
	}

	// Otherwise, look for an annotation-based identity.
	if name, annotationExists := filterMapValueByKey(pod.Annotations, annotationIdentityName); annotationExists && name != "" {
		klog.V(3).Infof("ziti identity name from annotations is %s", name)
		if err := zh.ZC.patchIdentityRoleAttributes(ctx, name, zh.Config.RoleKey, pod, oldPod); err != nil {
			return failureResponse(response, err)
		}
		return successResponse(response)
	}

	klog.V(3).Infof("no ziti identity name annotation or tunnel sidecar found for pod name '%s'", pod.Name)

	return successResponse(response)
}

func (cc *clusterClient) findNamespaceByOption(ctx context.Context, name string, opts metav1.ListOptions) (bool, error) {

	namespaces, err := cc.client.CoreV1().Namespaces().List(ctx, opts)
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

func (cc *clusterClient) getClusterService(ctx context.Context, namespace string, name string, opt metav1.GetOptions) (*corev1.Service, error) {
	return cc.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (cc *clusterClient) getPvcByOption(ctx context.Context, namespace string, name string, opt metav1.GetOptions) (*corev1.PersistentVolumeClaim, error) {
	return cc.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (cc *clusterClient) deletePvc(ctx context.Context, namespace string, name string) error {
	return cc.client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// create a ziti identity with a conventional name from the prefix, pod metadta, and admission request uid
func (zc *zitiClient) createIdentity(ctx context.Context, name string, roleKey string, podMeta *metav1.ObjectMeta) (string, error) {
	roles, ok := filterMapValueListByKey(
		podMeta.Annotations,
		roleKey,
	)
	if !ok {
		roles = []string{podMeta.Labels[labelApp]}
	}

	identityDetails, err := zitiedge.CreateIdentity(
		name,
		roles,
		rest_model_edge.IdentityTypeDevice,
		zc.client,
	)
	if err != nil {
		return "", err
	}

	return identityDetails.GetPayload().Data.ID, nil
}

// get the token for the identity by name or id
func (zc *zitiClient) getIdentityToken(ctx context.Context, name string, id string) (string, error) {

	if id == "" && name != "" {

		// returns nil or list of exactly one identity
		identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
		if err != nil {
			return "", err
		}

		// get the id from the list of one identity
		for _, identityItem := range identityDetails.GetPayload().Data {
			id = *identityItem.ID
		}
	}

	if id == "" {
		return "", errors.New("need name or id to get identity token")
	}

	// get the token for the identity by id
	detailsById, err := zitiedge.GetIdentityById(id, zc.client)
	if err != nil {
		return "", err
	}

	return detailsById.GetPayload().Data.Enrollment.Ott.JWT, nil
}

func (zc *zitiClient) deleteIdentity(ctx context.Context, name string) error {

	id := ""
	identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
	if err != nil {
		return err
	}

	for _, identityItem := range identityDetails.GetPayload().Data {
		id = *identityItem.ID
	}

	if id != "" {
		if err := zitiedge.DeleteIdentity(id, zc.client); err != nil {
			return err
		}
	}
	return nil
}

func (zc *zitiClient) findIdentityId(ctx context.Context, name string) (string, error) {

	id := ""
	identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
	if err != nil {
		return "", err
	}

	for _, identityItem := range identityDetails.GetPayload().Data {
		id = *identityItem.ID
	}

	return id, nil
}

func (zc *zitiClient) patchIdentityRoleAttributes(ctx context.Context, name string, key string, newPod *corev1.Pod, oldPod *corev1.Pod) error {

	var roles []string
	id := ""

	newRoles, newOk := filterMapValueListByKey(
		newPod.Annotations,
		key,
	)

	oldRoles, oldOk := filterMapValueListByKey(
		oldPod.Annotations,
		key,
	)

	if !newOk && oldOk {
		roles = []string{newPod.Labels["app"]}
	} else if newOk && !reflect.DeepEqual(newRoles, oldRoles) {
		roles = newRoles
	} else {
		roles = []string{}
	}

	identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
	if err != nil {
		return err
	}

	for _, identityItem := range identityDetails.GetPayload().Data {
		id = *identityItem.ID
	}

	if id != "" {
		if _, err := zitiedge.PatchIdentity(id, roles, zc.client); err != nil {
			return err
		}
	}
	return nil
}

func (zc *zitiClient) getZitiRouterToken(ctx context.Context, name string) (string, error) {

	routerDetails, err := zitiedge.GetEdgeRouterByName(name, zc.client)
	if err != nil {
		return "", err
	}
	if len(routerDetails.GetPayload().Data) > 0 {
		for _, routerItem := range routerDetails.GetPayload().Data {
			if *routerItem.EnrollmentJWT != "" {
				return *routerItem.EnrollmentJWT, nil
			} else {
				_, err := zitiedge.ReEnrollEdgeRouter(*routerItem.ID, zc.client)
				if err != nil {
					return "", err
				}

			}
		}
	}
	return "", nil
}

func (zc *zitiClient) updateZitiRouter(ctx context.Context, name string, options *rest_model_edge.EdgeRouterCreate) (*edge_router.CreateEdgeRouterCreated, error) {

	routerDetails, err := zitiedge.GetEdgeRouterByName(name, zc.client)
	if err != nil {
		return nil, err
	}
	if len(routerDetails.GetPayload().Data) == 0 {
		routerDetails, err := zitiedge.CreateEdgeRouter(options, zc.client)
		if err != nil {
			return nil, err
		}
		return routerDetails, nil
	}
	return nil, nil
}

func (zc *zitiClient) deleteZitiRouter(ctx context.Context, name string) error {

	routerDetails, err := zitiedge.GetEdgeRouterByName(name, zc.client)
	if err != nil {
		return err
	}
	for _, routerItem := range routerDetails.GetPayload().Data {
		if *routerItem.ID != "" {
			err = zitiedge.DeleteEdgeRouter(*routerItem.ID, zc.client)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// NewZitiHandler creates a new Ziti Handler.
func newZitiHandler(cc *clusterClient, zc *zitiClient, config *zitiConfig) *zitiHandler {
	return &zitiHandler{
		KC:     cc,
		ZC:     zc,
		Config: config,
	}
}
