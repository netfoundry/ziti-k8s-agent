package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"k8s.io/apimachinery/pkg/types"

	zitiedge "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"
	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_management_api_client/edge_router"
	rest_model_edge "github.com/openziti/edge-api/rest_model"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	rootUser     int64 = 0
	isNotTrue    bool  = false
	isPrivileged bool  = false
	jsonPatch    []JsonPatchEntry
)

const (
	volumeMountName string = "ziti-identity"

	// Annotation key for explicitly setting identity name
	annotationIdentityName = "identity.openziti.io/name"

	// Label keys in order of precedence
	labelApp          = "app"
	labelAppName      = "app.kubernetes.io/name"
	labelAppInstance  = "app.kubernetes.io/instance"
	labelAppComponent = "app.kubernetes.io/component"

	warnEmptyJWT            = "pod created without ziti enrollment token"
	warnIdentityNameExtract = "ziti identity name not found in pod annotations"
	warnIdentityListFailed  = "ziti identity list operation failed in ziti edge"

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
}

type zitiClient struct {
	client *rest_management_api_client.ZitiEdgeManagement
}

type zitiClientIntf interface {
	createIdentity(ctx context.Context, uid types.UID, prefix string, key string, pod *corev1.Pod) (string, string, error)
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
func (zh *zitiHandler) handleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
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
	ctx := context.Background()

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
					pod,
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

func (zh *zitiHandler) handleTunnelCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	identityName, identityId, err := zh.ZC.createIdentity(
		ctx,
		uid,
		zh.Config.Prefix,
		zh.Config.RoleKey,
		pod,
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

	// get cluster dns ip
	defaultClusterDnsServiceIP := "10.96.0.10"
	if len(zh.Config.ResolverIp) == 0 {
		service, err := zh.KC.getClusterService(
			ctx,
			"kube-system", "kube-dns",
			metav1.GetOptions{},
		)
		if err != nil {
			klog.Error(err)
		}
		if len(service.Spec.ClusterIP) != 0 {
			zh.Config.ResolverIp = service.Spec.ClusterIP
		} else {
			zh.Config.ResolverIp = defaultClusterDnsServiceIP
			klog.Warningf("Failed to look up DNS SVC ClusterIP, using default: %s", defaultClusterDnsServiceIP)
		}
	}

	dnsConfig := &corev1.PodDNSConfig{}
	if len(searchDomains) == 0 {
		dnsConfig = &corev1.PodDNSConfig{
			Nameservers: []string{
				"127.0.0.1",
				zh.Config.ResolverIp,
			},
			Searches: []string{
				"cluster.local",
				fmt.Sprintf("%s.svc", pod.Namespace),
			},
		}
	} else {
		dnsConfig = &corev1.PodDNSConfig{
			Nameservers: []string{
				"127.0.0.1",
				zh.Config.ResolverIp,
			},
			Searches: searchDomains,
		}
	}

	jsonPatch = []JsonPatchEntry{

		{
			OP:   "add",
			Path: "/spec/containers/-",
			Value: corev1.Container{
				Name:            identityName,
				Image:           fmt.Sprintf("%s:%s", zh.Config.Image, zh.Config.ImageVersion),
				ImagePullPolicy: corev1.PullPolicy(zh.Config.ImagePullPolicy),
				Args: []string{"tproxy"},
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

	patchBytes, err := json.Marshal(&jsonPatch)
	if err != nil {
		klog.Error(err)
	}

	response.Patch = patchBytes
	pt := admissionv1.PatchTypeJSONPatch
	response.PatchType = &pt
	return successResponse(response)
}

func (zh *zitiHandler) handleRouterCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	options := &rest_model_edge.EdgeRouterCreate{
		AppData:           nil,
		Cost:              &zh.Config.RouterConfig.Cost,
		Disabled:          &zh.Config.RouterConfig.Disabled,
		IsTunnelerEnabled: zh.Config.RouterConfig.IsTunnelerEnabled,
		Name:              &pod.Name,
		RoleAttributes:    &zh.Config.RouterConfig.RoleAttributes,
		Tags:              nil,
	}

	_, err = zh.ZC.updateZitiRouter(
		ctx,
		pod.Name,
		options,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	identityToken, err := zh.ZC.getZitiRouterToken(
		ctx,
		pod.Name,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	jsonPatch = []JsonPatchEntry{
		{
			OP:   "add",
			Path: "/spec/containers/0/env/-",
			Value: corev1.EnvVar{
				Name:  "ZITI_ENROLL_TOKEN",
				Value: identityToken,
			},
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

		if err := zh.ZC.deleteZitiRouter(ctx, pod.Name); err != nil {
			return failureResponse(response, err)
		}

	} else {

		name, ok := hasContainer(pod.Spec.Containers, zh.Config.Prefix)
		if ok {
			if err := zh.ZC.deleteIdentity(ctx, name); err != nil {
				return failureResponse(response, err)
			}
		} else {
			klog.Errorf("Not trying to delete ziti identity because there are no containers with prefix '%s'", zh.Config.Prefix)
		}

	}

	return successResponse(response)
}

func (zh *zitiHandler) handleUpdate(ctx context.Context, pod *corev1.Pod, oldPod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	name, ok := hasContainer(pod.Spec.Containers, zh.Config.Prefix)
	if ok {
		if err := zh.ZC.patchIdentityRoleAttributes(ctx, name, zh.Config.RoleKey, pod, oldPod); err != nil {
			return failureResponse(response, err)
		}
	}
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

func buildZitiIdentityName(prefix string, pod *corev1.Pod, uid types.UID) (string, error) {
	var name string

	// Check for explicit annotation first
	if annotatedName, exists := pod.Annotations[annotationIdentityName]; exists && annotatedName != "" {
		name = annotatedName
	} else {
		// Check labels in order of precedence
		labels := []string{labelApp, labelAppName, labelAppInstance, labelAppComponent}
		for _, label := range labels {
			if labelName, exists := pod.Labels[label]; exists && labelName != "" {
				name = labelName
				break
			}
		}
	}

	if name == "" {
		return "", fmt.Errorf("failed to build identity name: no valid name found in annotations or labels")
	}

	// Build base name with prefix, name and namespace
	baseName := fmt.Sprintf("%s-%s-%s", prefix, name, pod.Namespace)

	// Truncate to 50 characters if needed
	if len(baseName) > 50 {
		baseName = baseName[:50]
	}

	// Create SHA256 hash of UID and truncate to 10 characters
	hasher := sha256.New()
	hasher.Write([]byte(string(uid)))
	hash := hex.EncodeToString(hasher.Sum(nil))[:10]

	// Build final identity name with hash suffix
	identityName := fmt.Sprintf("%s-%s", baseName, hash)

	// Validate the final name
	valid, err := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`, identityName)
	if err != nil {
		return "", fmt.Errorf("error validating identity name: %v", err)
	}
	if !valid {
		return "", fmt.Errorf("invalid identity name format: %s", identityName)
	}

	return identityName, nil
}

func (zc *zitiClient) createIdentity(ctx context.Context, uid types.UID, prefix string, roleKey string, pod *corev1.Pod) (string, string, error) {

	name, err := buildZitiIdentityName(sidecarPrefix, pod, uid)
	roles, ok := filterMapValuesByKey(
		pod.Annotations,
		roleKey,
	)
	if !ok {
		roles = []string{pod.Labels["app"]}
	}

	identityDetails, err := zitiedge.CreateIdentity(
		name,
		roles,
		"Device",
		zc.client,
	)
	if err != nil {
		return "", "", err
	}

	return name, identityDetails.GetPayload().Data.ID, nil
}

func (zc *zitiClient) getIdentityToken(ctx context.Context, name string, id string) (string, error) {

	if id == "" {

		identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
		if err != nil {
			return "", err
		}

		for _, identityItem := range identityDetails.GetPayload().Data {
			id = *identityItem.ID
		}
	}

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

	newRoles, newOk := filterMapValuesByKey(
		newPod.Annotations,
		key,
	)

	oldRoles, oldOk := filterMapValuesByKey(
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
		routerDetails, err := zitiedge.CreateEdgeRouter(name, options, zc.client)
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
