package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	k "github.com/netfoundry/ziti-k8s-agent/ziti-agent/kubernetes"
	ze "github.com/netfoundry/ziti-k8s-agent/ziti-agent/ziti-edge"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/sdk-golang/ziti"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	volumeMountName string = "sidecar-ziti-identity"

	// Annotation key for explicitly setting identity name
	annotationIdentityName = "identity.openziti.io/name"

	// Label keys in order of precedence
	labelApp          = "app"
	labelAppName      = "app.kubernetes.io/name"
	labelAppInstance  = "app.kubernetes.io/instance"
	labelAppComponent = "app.kubernetes.io/component"
)

type JsonPatchEntry struct {
	OP    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

// zitiTunnel handles Kubernetes admission requests for pod operations.
// It processes "CREATE", "DELETE", and "UPDATE" operations to manage Ziti identities
// and associated Kubernetes resources based on pod annotations and labels.
//
// Args:
//   ar: AdmissionReview object containing the admission request details.
//
// Returns:
//   A pointer to the AdmissionResponse indicating success or failure
//   of the admission request processing.
func zitiTunnel(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}

	// parse ziti admin certs
	zitiTlsCertificate, err := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	if len(zitiTlsCertificate.Certificate) == 0 {
		err := fmt.Errorf("no certificates found in TLS key pair")
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	parsedCert, err := x509.ParseCertificate(zitiTlsCertificate.Certificate[0])
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	zecfg := ze.Config{ApiEndpoint: zitiCtrlMgmtApi, Cert: parsedCert, PrivateKey: zitiTlsCertificate.PrivateKey}

	klog.Infof("%s operation admission request UID: %s", ar.Request.Operation, ar.Request.UID)
	switch ar.Request.Operation {

	case "CREATE":
		klog.V(4).Infof("Object: %s", ar.Request.Object.Raw)
		klog.V(4).Infof("OldObject: %s", ar.Request.OldObject.Raw)
		if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &pod); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}

		identityName, err := buildZitiIdentityName(sidecarPrefix, &pod)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		klog.V(4).Infof("identity name is %s", identityName)
		klog.V(4).Infof("Pod Labels are %s", pod.Labels)
		klog.V(4).Infof("Pod Annotations are %s", pod.Annotations)

		roles, ok := getIdentityAttributes(pod.Annotations)
		if !ok {
			appLabel, exists := pod.Labels["app"]
			if !exists {
				err := fmt.Errorf("pod must have either ziti role annotation or an 'app' label")
				klog.Error(err)
				return failureResponse(reviewResponse, err)
			}
			roles = []string{appLabel}
		}

		zec, err := ze.Client(&zecfg)
		if err != nil {
			return failureResponse(reviewResponse, err)
		}

		klog.V(4).Infof("Pod Name is %s", pod.Name)
		klog.V(4).Infof("Pod Namespace is %s", pod.Namespace)

		identityCfg, _, err := createAndEnrollIdentity(identityName, roles, zec)
		if err != nil {
			return failureResponse(reviewResponse, err)
		}

		secretJson, err := json.Marshal(identityCfg)
		if err != nil {
			klog.Error(err)
			return failureResponse(reviewResponse, err)
		}
		klog.V(5).Infof("Enrolled identity cfg is '%s'", string(secretJson))

		// kubernetes client
		kc := k.Client()

		//Create secret in the same namespace
		_, err = kc.CoreV1().Secrets(pod.Namespace).Create(
			context.TODO(), &corev1.Secret{
				Data: map[string][]byte{identityName: secretJson},
				Type: "Opaque",
				ObjectMeta: metav1.ObjectMeta{
					Name: identityName,
				},
			}, metav1.CreateOptions{},
		)
		if err != nil {
			klog.Error(err)
		}

		if len(clusterDnsServiceIP) == 0 {
			dnsService, err := kc.CoreV1().Services("kube-system").Get(context.TODO(), "kube-dns", metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
			}
			if len(dnsService.Spec.ClusterIP) != 0 {
				clusterDnsServiceIP = dnsService.Spec.ClusterIP
				klog.Infof("Looked up DNS SVC ClusterIP: %s", dnsService.Spec.ClusterIP)
			} else {
				klog.Info("Looked up DNS SVC ClusterIP and is not found")
			}
		}

		// add sidecar volume to pod
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: identityName,
					Items:      []corev1.KeyToPath{{Key: identityName, Path: fmt.Sprintf("%v.json", identityName)}},
				},
			},
		})

		volumesBytes, err := json.Marshal(&pod.Spec.Volumes)
		if err != nil {
			klog.Error(err)
		}

		// update pod dns config and policy
		if len(searchDomains) == 0 {
			pod.Spec.DNSConfig = &corev1.PodDNSConfig{
				Nameservers: []string{"127.0.0.1", clusterDnsServiceIP},
				Searches:    []string{"cluster.local", fmt.Sprintf("%s.svc", pod.Namespace)},
			}
		} else {
			pod.Spec.DNSConfig = &corev1.PodDNSConfig{
				Nameservers: []string{"127.0.0.1", clusterDnsServiceIP},
					Searches:    searchDomains,
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

		var podSecurityContextBytes []byte
		var patch []JsonPatchEntry
		var rootUser int64 = 0
		var isNotTrue bool = false
		var isPrivileged = false
		var sidecarSecurityContext *corev1.SecurityContext

		sidecarSecurityContext = &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add:  []corev1.Capability{"NET_ADMIN", "NET_BIND_SERVICE"},
				Drop: []corev1.Capability{"ALL"},
			},
			RunAsUser:  &rootUser,
			Privileged: &isPrivileged,
		}

		if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil {
			sidecarSecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add:  []corev1.Capability{"NET_ADMIN", "NET_BIND_SERVICE"},
					Drop: []corev1.Capability{"ALL"},
				},
				RunAsUser:  &rootUser,
				Privileged: &isPrivileged,
			}
		}

		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:            identityName,
			Image:           fmt.Sprintf("%s:%s", sidecarImage, sidecarImageVersion),
			Args:            []string{"tproxy", "-i", fmt.Sprintf("%v.json", identityName)},
			VolumeMounts:    []corev1.VolumeMount{{Name: volumeMountName, MountPath: "/netfoundry", ReadOnly: true}},
			SecurityContext: sidecarSecurityContext,
		})

		containersBytes, err := json.Marshal(&pod.Spec.Containers)
		if err != nil {
			klog.Error(err)
		}

		patch = []JsonPatchEntry{

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

		// update Pod Security Context RunAsNonRoot to false
		if podSecurityOverride {
			pod.Spec.SecurityContext.RunAsNonRoot = &isNotTrue
			podSecurityContextBytes, err = json.Marshal(&pod.Spec.SecurityContext)
			if err != nil {
				klog.Error(err)
			}
			patch = append(patch, []JsonPatchEntry{
				{
					OP:    "replace",
					Path:  "/spec/securityContext",
					Value: podSecurityContextBytes,
				},
			}...)
		}

		patchBytes, err := json.Marshal(&patch)
		if err != nil {
			klog.Error(err)
		}

		reviewResponse.Patch = patchBytes
		// klog.Infof(fmt.Sprintf("Patch bytes: %s", reviewResponse.Patch))
		pt := admissionv1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt

	case "DELETE":
		if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, &pod); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}

		identityName, err := buildZitiIdentityName(sidecarPrefix, &pod)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		klog.V(4).Infof("Identity name is %s", identityName)

		// kubernetes client
		kc := k.Client()
		secretData, err := kc.CoreV1().Secrets(pod.Namespace).Get(context.TODO(), identityName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
		}
		if len(secretData.Name) > 0 {
			err = kc.CoreV1().Secrets(pod.Namespace).Delete(context.TODO(), identityName, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
			} else {
				klog.Infof("Deleted secret '%s'", identityName)
			}
		} else {
			klog.Infof("Secret '%s' already deleted", identityName)
		}

		zec, err := ze.Client(&zecfg)
		if err != nil {
			return failureResponse(reviewResponse, err)
		}

		zId, found, err := findIdentity(identityName, zec)
		if err != nil {
			return failureResponse(reviewResponse, err)
		}
		if found {
			err = ze.DeleteIdentity(zId, zec)
			if err != nil {
				return failureResponse(reviewResponse, err)
			}
			klog.V(4).Infof("Deleted identity '%s' with id '%s'", identityName, zId)
		} else {
			klog.V(4).Infof("Identity '%s' already deleted", identityName)
		}

	case "UPDATE":
		klog.Infof("Starting webhook operation: %s", ar.Request.Operation)
		klog.V(5).Infof("Object: %s", ar.Request.Object.Raw)
		klog.V(5).Infof("OldObject: %s", ar.Request.OldObject.Raw)
		if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &pod); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, &oldPod); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}

		identityName, err := buildZitiIdentityName(sidecarPrefix, &pod)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		klog.V(4).Infof("identity name is %s", identityName)

		var roles []string
		klog.Infof("Pod Annotations are %v", pod.Annotations)
		newRoles, newOk := getIdentityAttributes(pod.Annotations)
		klog.Infof("OldPod Annotations are %v", oldPod.Annotations)
		oldRoles, oldOk := getIdentityAttributes(oldPod.Annotations)

		if !newOk && oldOk {
			// Ziti Annotation is removed
			roles = []string{pod.Labels["app"]}
		} else if newOk && !reflect.DeepEqual(newRoles, oldRoles) {
			//Ziti Annotation is created or updated
			roles = newRoles
		} else {
			roles = []string{}
		}

		klog.V(4).Infof("Roles are %v", roles)
		klog.V(4).Infof("Roles length is %d", len(roles))
		// Update only if Ziti Annotation is changed
		if len(roles) > 0 {
			zec, err := ze.Client(&zecfg)
			if err != nil {
				return failureResponse(reviewResponse, err)
			}
			zId, found, err := findIdentity(identityName, zec)
			if err != nil {
				klog.Error(err)
				return failureResponse(reviewResponse, err)
			}
			if found {
				identityDetails, err := ze.PatchIdentity(zId, roles, zec)
				if err != nil {
					return failureResponse(reviewResponse, err)
				}
				klog.V(5).Infof("Updated identity details are %v", identityDetails)
			} else {
				klog.Errorf("Identity '%s' not found during pod update operation", identityName)
			}
		}
	}
	return successResponse(reviewResponse)
}

// createAndEnrollIdentity creates a new identity in Ziti Edge with the given name, UID and roles,
// enrolls it and returns the enrolled configuration and the name of the created identity.
//
// Args:
//   name: The name of the identity to create.
//   uid: The UID of the pod for which the identity is created.
//   roles: A list of roles to assign to the created identity.
//   zec: A reference to the Ziti Edge client.
//
// Returns:
//   A pointer to a ziti.Config containing the enrolled configuration if successful, otherwise nil.
//   A string containing the name of the created identity.
//   An error if any occurs.
func createAndEnrollIdentity(name string, roles []string, zec *rest_management_api_client.ZitiEdgeManagement) (*ziti.Config, string, error) {
	klog.V(4).Infof("creating identity %s", name)

	identityDetails, err := ze.CreateIdentity(name, roles, "Device", zec)
	if err != nil {
		klog.Error(err)
		return nil, name, err
	}

	identityCfg, err := ze.EnrollIdentity(identityDetails.GetPayload().Data.ID, zec)
	if err != nil {
		klog.Error(err)
		return nil, name, err
	}

	return identityCfg, name, nil
}

// findIdentity looks up an identity by name and returns its ID if found.
//
// Args:
//   name: The name of the identity to look up.
//   zec: A reference to the Ziti Edge client.
//
// Returns:
//   A string containing the ID of the identity if found, otherwise an empty string.
//   A boolean indicating whether the identity was found.
//   An error object if an error occurred.
func findIdentity(name string, zec *rest_management_api_client.ZitiEdgeManagement) (string, bool, error) {
	identityDetails, err := ze.GetIdentityByName(name, zec)
	if err != nil {
		klog.Error(err)
		return "", false, err
	}

	data := identityDetails.GetPayload().Data
	if len(data) == 0 {
		klog.V(4).Infof("No identity found with name: %s", name)
		return "", false, nil
	}

	if len(data) > 1 {
		klog.Warningf("Multiple identities found with name %s. Using first match.", name)
	}

	zId := *data[0].ID
	klog.V(4).Infof("Found identity %s with name: %s", zId, name)
	return zId, true, nil
}

// getIdentityAttributes extracts the role attributes from the given roles map.
//
// If the given roles map contains a key matching the value of zitiRoleKey, it
// is expected to have a value that is a comma-separated list of role attributes.
// If the value is not empty, it is split into individual strings and returned.
// If the key is not present, or the value is empty, an empty list is returned
// and the boolean value is false.
//
// Args:
//   roles: A map of string key-value pairs.
//
// Returns:
//   A list of strings representing the role attributes, and a boolean indicating
//   whether the key was present in the roles map.
func getIdentityAttributes(roles map[string]string) ([]string, bool) {
	// if a ziti role key is not present, use app name as a role attribute
	value, ok := roles[zitiRoleKey]
	if ok {
		if len(value) > 0 {
			return strings.Split(value, ","), true
		}
	}
	return []string{}, false
}

// trimString is called when creating Ziti identity names and trims input to a maximum of 24 characters in length. If
// the string is longer than 24 characters, the first 24 characters are returned. Otherwise, the original string is
// returned.
func trimString(input string) string {
	if len(input) > 24 {
		return input[:24]
	}
	return input
}

func validateSubdomain(input string) error {
	_, err := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`, input)
	if err != nil {
		return err
	}
	return nil
}

func buildZitiIdentityName(prefix string, pod *corev1.Pod) (string, error) {
	var name string
	var isUID bool

	// Check for explicit annotation first
	if annotatedName, exists := pod.Annotations[annotationIdentityName]; exists && annotatedName != "" {
		name = annotatedName
	} else {
		// Check labels in order of precedence
		labels := []string{labelApp, labelAppName, labelAppInstance, labelAppComponent}
		for _, label := range labels {
			klog.V(4).Infof("Checking label %s=%s", label, pod.Labels[label])
			if labelName, exists := pod.Labels[label]; exists && labelName != "" {
				name = labelName
				klog.V(4).Infof("Set name from label %s=%s", label, name)
				break
			}
		}

		// Check pod name if no label was found
		if name == "" && pod.Name != "" {
			name = pod.Name
		}

		// Fall back to pod UID if nothing else is available
		if name == "" {
			return "", fmt.Errorf("failed to build identity name: set pod app label or pod.Name")
		}
	}

	// Trim the name if it's too long and not a UID
	if !isUID {
		name = trimString(name)
	}

	// Build the full identity name
	identityName := fmt.Sprintf("%s-%s-%s", prefix, name, pod.Namespace)

	// Validate the final name
	if err := validateSubdomain(identityName); err != nil {
		return "", fmt.Errorf("invalid identity name '%s': %v", identityName, err)
	}

	return identityName, nil
}

// failureResponse sets the admission response as a failure with the provided error.
//
// Args:
//   ar: The admissionv1.AdmissionResponse to be updated.
//   err: The error that occurred, which will be logged and included in the response reason.
//
// Returns:
//   A pointer to the updated admissionv1.AdmissionResponse with Allowed set to false,
//   and the Result status set to "Failure" with a reason including the error message.
func failureResponse(ar admissionv1.AdmissionResponse, err error) *admissionv1.AdmissionResponse {
	klog.Error(err)
	ar.Allowed = false
	ar.Result = &metav1.Status{
		Status: "Failure",
		Reason: metav1.StatusReason(fmt.Sprintf("Ziti Controller -  %s", err)),
	}
	return &ar
}

// successResponse sets the admission response as a success.
//
// Args:
//   ar: The admissionv1.AdmissionResponse to be updated.
//
// Returns:
//   A pointer to the updated admissionv1.AdmissionResponse with Allowed set to true,
//   and the Result status set to "Success".
func successResponse(ar admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {
	ar.Allowed = true
	ar.Result = &metav1.Status{
		Status: "Success",
	}
	return &ar
}
