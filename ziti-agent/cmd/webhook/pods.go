package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/types"

	zitiedge "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"
	"github.com/openziti/edge-api/rest_management_api_client"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	rootUser                int64 = 0
	isNotTrue               bool  = false
	isPrivileged            bool  = false
	podSecurityContextBytes []byte
	jsonPatch               []JsonPatchEntry
	podSecurityContext      *corev1.SecurityContext
)

type clusterClient struct {
	client *kubernetes.Clientset
}

type clusterClientIntf interface {
	getClusterService(ctx context.Context, namespace string, name string, opts metav1.GetOptions) (*corev1.Service, error)
	findNamespaceByOption(ctx context.Context, name string, opts metav1.ListOptions) (bool, error)
}

type zitiClient struct {
	client *rest_management_api_client.ZitiEdgeManagement
	err    error
}

type zitiClientIntf interface {
	createIdentity(ctx context.Context, uid types.UID, prefix string, key string, pod *corev1.Pod) (string, string, error)
	deleteIdentity(ctx context.Context, id string) error
	getIdentityToken(ctx context.Context, name string, id string) (string, error)
	patchIdentityRoleAttributes(ctx context.Context, id string, key string, newPod *corev1.Pod, oldPod *corev1.Pod) error
}

type zitiConfig struct {
	Image           string
	ImageVersion    string
	VolumeMountName string
	Prefix          string
	RoleKey         string
	LabelKey        string
	labelDelValue   string
	labelCrValue    string
	resolverIp      string
}

type zitiHandler struct {
	KC     clusterClientIntf
	ZC     zitiClientIntf
	Config *zitiConfig
}

type ZitiHandlerIntf interface{}

// HandleAdmissionRequest handles the admission request for old pod types, i.e. tunnel, router, etc.
func (zh *zitiHandler) handleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := &corev1.Pod{}
	oldPod := &corev1.Pod{}

	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, pod); err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, err)
	}
	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, oldPod); err != nil {
		klog.Error(err)
		return failureResponse(reviewResponse, err)
	}

	// klog.Infof("Admission Request UID: %s", ar.Request.UID)
	// klog.Infof("Admission Request Operation: %s", ar.Request.Operation)
	// klog.Infof("Admission Request App Name %s, OldApp Name: %s", pod.Labels["app"], oldPod.Labels["app"])

	deleteLabelFound, err := zh.KC.findNamespaceByOption(
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
		if !deleteLabelFound {
			// klog.Infof("Creating: delete action %v", deleteLabelFound)
			return zh.handleCreate(
				context.Background(),
				pod,
				ar.Request.UID,
				reviewResponse,
			)
		}
	case "DELETE":
		// klog.Infof("Deleting: delete action %v", deleteLabelFound)
		return zh.handleDelete(
			context.Background(),
			oldPod,
			reviewResponse,
		)
	case "UPDATE":
		if !deleteLabelFound {
			// klog.Infof("Updating: delete action %v", deleteLabelFound)
			return zh.handleUpdate(
				context.Background(),
				pod,
				oldPod,
				reviewResponse,
			)
		}
	}

	return successResponse(reviewResponse)
}

func (zh *zitiHandler) handleCreate(ctx context.Context, pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	identityName, identityId, err := zh.ZC.createIdentity(
		context.Background(),
		uid,
		zh.Config.Prefix,
		zh.Config.RoleKey,
		pod,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	identityToken, err := zh.ZC.getIdentityToken(
		context.Background(),
		identityName,
		identityId,
	)
	if err != nil {
		return failureResponse(response, err)
	}

	if len(zh.Config.resolverIp) == 0 {
		service, err := zh.KC.getClusterService(
			context.Background(),
			"kube-system", "kube-dns",
			metav1.GetOptions{},
		)
		if err != nil {
			klog.Error(err)
		}
		if len(service.Spec.ClusterIP) != 0 {
			zh.Config.resolverIp = service.Spec.ClusterIP
		} else {
			klog.Info("Looked up DNS SVC ClusterIP and is not found")
		}
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: zh.Config.VolumeMountName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumesBytes, err := json.Marshal(&pod.Spec.Volumes)
	if err != nil {
		klog.Error(err)
	}

	if len(searchDomains) == 0 {
		pod.Spec.DNSConfig = &corev1.PodDNSConfig{
			Nameservers: []string{
				"127.0.0.1",
				zh.Config.resolverIp,
			},
			Searches: []string{
				"cluster.local",
				fmt.Sprintf("%s.svc", pod.Namespace),
			},
		}
	} else {
		pod.Spec.DNSConfig = &corev1.PodDNSConfig{
			Nameservers: []string{
				"127.0.0.1",
				zh.Config.resolverIp,
			},
			Searches: searchDomains,
		}
	}

	dnsConfigBytes, err := json.Marshal(&pod.Spec.DNSConfig)
	if err != nil {
		klog.Error(err)
	}

	pod.Spec.DNSPolicy = "None"
	dnsPolicyBytes, err := json.Marshal(&pod.Spec.DNSPolicy)
	if err != nil {
		klog.Error(err)
	}

	podSecurityContext = &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Add:  []corev1.Capability{"NET_ADMIN", "NET_BIND_SERVICE"},
			Drop: []corev1.Capability{"ALL"},
		},
		RunAsUser:  &rootUser,
		Privileged: &isPrivileged,
	}

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            identityName,
		Image:           fmt.Sprintf("%s:%s", zh.Config.Image, zh.Config.ImageVersion),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"tproxy",
			"-i",
			fmt.Sprintf("%v.json", identityName),
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ZITI_ENROLL_TOKEN",
				Value: identityToken,
			},
			{
				Name:  "NF_REG_NAME",
				Value: identityName,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      zh.Config.VolumeMountName,
				MountPath: "/netfoundry",
				ReadOnly:  false,
			},
		},
		SecurityContext: podSecurityContext,
	})

	containersBytes, err := json.Marshal(&pod.Spec.Containers)
	if err != nil {
		klog.Error(err)
	}

	jsonPatch = []JsonPatchEntry{

		{
			OP:    "add",
			Path:  "/spec/containers",
			Value: containersBytes,
		},
		{
			OP:    "add",
			Path:  "/spec/volumes",
			Value: volumesBytes,
		},
		{
			OP:    "replace",
			Path:  "/spec/dnsPolicy",
			Value: dnsPolicyBytes,
		},
		{
			OP:    "add",
			Path:  "/spec/dnsConfig",
			Value: dnsConfigBytes,
		},
	}

	if podSecurityOverride {
		pod.Spec.SecurityContext.RunAsNonRoot = &isNotTrue
		podSecurityContextBytes, err = json.Marshal(&pod.Spec.SecurityContext)
		if err != nil {
			klog.Error(err)
		}
		jsonPatch = append(jsonPatch, []JsonPatchEntry{
			{
				OP:    "replace",
				Path:  "/spec/securityContext",
				Value: podSecurityContextBytes,
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

func (zh *zitiHandler) handleDelete(ctx context.Context, pod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	name, ok := hasContainer(pod.Spec.Containers, fmt.Sprintf("%s-%s", pod.Labels["app"], zh.Config.Prefix))
	if ok {
		if err := zh.ZC.deleteIdentity(context.Background(), name); err != nil {
			return failureResponse(response, err)
		}
	} else {
		klog.Infof("Container %s not found in Pod %s", name, pod.Name)
	}

	return successResponse(response)
}

func (zh *zitiHandler) handleUpdate(ctx context.Context, pod *corev1.Pod, oldPod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	name, ok := hasContainer(pod.Spec.Containers, fmt.Sprintf("%s-%s", trimString(pod.Labels["app"]), zh.Config.Prefix))
	if ok {
		if err := zh.ZC.patchIdentityRoleAttributes(context.Background(), name, zh.Config.RoleKey, pod, oldPod); err != nil {
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

func (zc *zitiClient) createIdentity(ctx context.Context, uid types.UID, prefix string, key string, pod *corev1.Pod) (string, string, error) {

	if zc.err != nil {
		return "", "", zc.err
	}
	name := fmt.Sprintf("%s-%s-%s", trimString(pod.Labels["app"]), prefix, uid)

	roles, ok := filterMapValuesByKey(
		pod.Annotations,
		key,
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

	if zc.err != nil {
		return "", zc.err
	}

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

	if zc.err != nil {
		return zc.err
	}

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

func (zc *zitiClient) patchIdentityRoleAttributes(ctx context.Context, name string, key string, newPod *corev1.Pod, oldPod *corev1.Pod) error {

	var roles []string
	id := ""

	if zc.err != nil {
		return zc.err
	}

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

// NewZitiHandler creates a new Ziti Handler.
func newZitiHandler(cc *clusterClient, zc *zitiClient, config *zitiConfig) *zitiHandler {
	return &zitiHandler{
		KC:     cc,
		ZC:     zc,
		Config: config,
	}
}
