package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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
	deleteIdentity(id string) error
	findIdentity(name string) (string, error)
	getIdentityAttributes(roles map[string]string, key string) ([]string, bool)
	getIdentityToken(name string, prefix string, uid types.UID, roles []string) (string, string, error)
	patchIdentityRoles(id string, roles []string) error
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

type ZitiHandler interface {
	handleAdmissionRequest(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

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
			return zh.handleCreate(pod, ar.Request.UID, reviewResponse)
		}
	case "DELETE":
		// klog.Infof("Deleting: delete action %v", deleteLabelFound)
		return zh.handleDelete(oldPod, reviewResponse)
	case "UPDATE":
		if !deleteLabelFound {
			// klog.Infof("Updating: delete action %v", deleteLabelFound)
			return zh.handleUpdate(pod, oldPod, reviewResponse)
		}
	}

	return successResponse(reviewResponse)
}

func (zh *zitiHandler) handleCreate(pod *corev1.Pod, uid types.UID, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	roles, ok := zh.ZC.getIdentityAttributes(
		pod.Annotations,
		zh.Config.RoleKey,
	)
	if !ok {
		roles = []string{pod.Labels["app"]}
	}

	identityToken, identityName, err := zh.ZC.getIdentityToken(
		pod.Labels["app"],
		zh.Config.Prefix,
		uid,
		roles,
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
				Name:      volumeMountName,
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

func (zh *zitiHandler) handleDelete(pod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	name, ok := hasContainer(pod.Spec.Containers, fmt.Sprintf("%s-%s", pod.Labels["app"], zh.Config.Prefix))
	if ok {
		if err := zh.ZC.deleteIdentity(name); err != nil {
			return failureResponse(response, err)
		}
	} else {
		klog.Infof("Container %s not found in Pod %s", name, pod.Name)
	}

	return successResponse(response)
}

func (zh *zitiHandler) handleUpdate(pod *corev1.Pod, oldPod *corev1.Pod, response admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {

	name, ok := hasContainer(pod.Spec.Containers, fmt.Sprintf("%s-%s", pod.Labels["app"], sidecarPrefix))
	if ok {
		var roles []string
		// klog.Infof("Pod Annotations are %s", pod.Annotations)
		newRoles, newOk := zh.ZC.getIdentityAttributes(
			pod.Annotations,
			zh.Config.RoleKey,
		)
		// klog.Infof("OldPod Annotations are %s", oldPod.Annotations)
		oldRoles, oldOk := zh.ZC.getIdentityAttributes(
			oldPod.Annotations,
			zh.Config.RoleKey,
		)

		if !newOk && oldOk {
			roles = []string{pod.Labels["app"]}
		} else if newOk && !reflect.DeepEqual(newRoles, oldRoles) {
			roles = newRoles
		} else {
			roles = []string{}
		}

		// klog.Infof("Roles length is %d", len(roles))

		if len(roles) > 0 {
			if err := zh.ZC.patchIdentityRoles(name, roles); err != nil {
				return failureResponse(response, err)
			}
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

func (zc *zitiClient) getIdentityToken(name string, prefix string, uid types.UID, roles []string) (string, string, error) {

	if zc.err != nil {
		return "", "", zc.err
	}

	identityName := fmt.Sprintf("%s-%s%s", trimString(name), prefix, uid)

	identityDetails, err := zitiedge.CreateIdentity(identityName, roles, "Device", zc.client)
	if err != nil {
		klog.Error(err)
		return "", identityName, err
	}

	identityToken, err := zitiedge.GetIdentityById(identityDetails.GetPayload().Data.ID, zc.client)
	if err != nil {
		klog.Error(err)
		return "", identityName, err
	}
	return identityToken.GetPayload().Data.Enrollment.Ott.JWT, identityName, nil
}

func (zc *zitiClient) getIdentityAttributes(roles map[string]string, key string) ([]string, bool) {

	value, ok := roles[key]
	if ok {
		if len(value) > 0 {
			return strings.Split(value, ","), true
		}
	}
	return []string{}, false
}

func (zc *zitiClient) findIdentity(name string) (string, error) {

	if zc.err != nil {
		return "", zc.err
	}

	identityDetails, err := zitiedge.GetIdentityByName(name, zc.client)
	if err != nil {
		return "", err
	}

	for _, identityItem := range identityDetails.GetPayload().Data {
		return *identityItem.ID, nil
	}
	return "", nil
}

func (zc *zitiClient) deleteIdentity(name string) error {

	if zc.err != nil {
		return zc.err
	}

	id, err := zc.findIdentity(name)
	if err != nil {
		return err
	}
	if id != "" {
		if err := zitiedge.DeleteIdentity(id, zc.client); err != nil {
			return err
		}
	}
	return nil
}

func (zc *zitiClient) patchIdentityRoles(name string, roles []string) error {

	if zc.err != nil {
		return zc.err
	}

	id, err := zc.findIdentity(name)
	if err != nil {
		return err
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
