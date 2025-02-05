package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	ze "github.com/netfoundry/ziti-k8s-agent/ziti-agent/ziti-edge"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	warnEmptyJWT        = "pod created without ziti enrollment token"
	warnIdentityNameExtract = "ziti identity name not found in pod annotations"
	warnIdentityListFailed = "ziti identity list operation failed in ziti edge"
)

type JsonPatchEntry struct {
	OP    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

// handleZitiTunnelAdmission handles Kubernetes admission requests for pod operations.
// It processes "CREATE", "DELETE", and "UPDATE" operations to manage Ziti identities
// and associated Kubernetes resources based on pod annotations and labels.
//
// Args:
//   ar: AdmissionReview object containing the admission request details.
//
// Returns:
//   A pointer to the AdmissionResponse indicating success or failure
//   of the admission request processing.
func handleZitiTunnelAdmission(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}

	// parse ziti admin certs to synchronously (blocking) create a ziti identity
	zitiAdminIdentity, err := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	if len(zitiAdminIdentity.Certificate) == 0 {
		err := fmt.Errorf("no certificates found in TLS key pair")
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	parsedCert, err := x509.ParseCertificate(zitiAdminIdentity.Certificate[0])
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	klog.V(4).Infof("Parsed client certificate - Subject: %v, Issuer: %v", parsedCert.Subject, parsedCert.Issuer)
	klog.V(4).Infof("Loading CA bundle, size: %d bytes", len(zitiCtrlCaBundle))
	klog.V(5).Infof("CA bundle content: %s", string(zitiCtrlCaBundle))
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(zitiCtrlCaBundle) {
		klog.V(2).Infof("CA bundle content: %s", string(zitiCtrlCaBundle))
		err := fmt.Errorf("failed to append CA certificates from PEM")
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	zecfg := ze.Config{
		ApiEndpoint: zitiCtrlMgmtApi,
		Cert:        parsedCert,
		PrivateKey:  zitiAdminIdentity.PrivateKey,
		CAS:         *certPool,
	}

	klog.Infof("%s operation admission request UID: %s", ar.Request.Operation, ar.Request.UID)
	switch ar.Request.Operation {

	case "CREATE":
		klog.V(4).Infof("Object: %s", ar.Request.Object.Raw)
		klog.V(4).Infof("OldObject: %s", ar.Request.OldObject.Raw)
		if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &pod); err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		identityName, err := buildZitiIdentityName(sidecarPrefix, &pod, ar.Request.UID)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		klog.V(4).Infof("deterministic identity name is %s", identityName)
		klog.V(4).Infof("Pod Labels are %s", pod.Labels)
		klog.V(4).Infof("Pod Annotations are %s", pod.Annotations)

		roles, ok := getIdentityAttributes(pod.Annotations)
		if !ok {
			appLabel, exists := pod.Labels["app"]
			if !exists {
				err := fmt.Errorf("pod must have either ziti role annotation or an 'app' label")
				klog.Error(err)
				reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
			}
			roles = []string{appLabel}
		}

		zec, err := ze.Client(&zecfg)
		if err != nil {
			errStack := errors.WithStack(err)
			klog.Errorf("Error with stack trace: %+v", errStack)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		klog.V(4).Infof("pod.Name is %s, pod.Namespace is %s", pod.Name, pod.Namespace)
		identityDetails, err := ze.CreateIdentity(identityName, roles, "Device", zec)
		if err != nil {
			klog.Errorf("failed to create identity %s: %v", identityName, err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		klog.V(4).Infof("created identity with details: %v", identityDetails)

		identityJwt, err := ze.GetIdentityEnrollmentJWT(identityDetails.GetPayload().Data.ID, zec)
		if err != nil {
			klog.Errorf("failed to get JWT for identity %s: %v", identityName, err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		klog.V(4).Infof("successfully created identity '%s'", identityName)
		klog.V(5).Infof("identity '%s' has JWT: '%s'", identityName, *identityJwt)

		// add identity dir empty dir volume to pod
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		// Store admission request UID in annotations for identity correlation
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[annotationIdentityName] = identityName
		annotationsBytes, err := json.Marshal(pod.Annotations)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		// create container env vars
		var containerEnv []corev1.EnvVar
		if identityJwt != nil {
			containerEnv = append(containerEnv, corev1.EnvVar{
				Name:  "ZITI_ENROLL_TOKEN",
				Value: *identityJwt,
			})
		} else {
			klog.V(4).Infof("skipping ZITI_ENROLL_TOKEN env var: JWT is nil for pod %s/%s", pod.Namespace, pod.Name)
			reviewResponse.Warnings = append(reviewResponse.Warnings, warnEmptyJWT)
		}

		volumesBytes, err := json.Marshal(&pod.Spec.Volumes)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
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
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		pod.Spec.DNSPolicy = "None"
		dnsPolicyBytes, err := json.Marshal(&pod.Spec.DNSPolicy)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		var podSecurityContextBytes []byte
		var patch []JsonPatchEntry
		var rootUser int64 = 0
		var isNotTrue bool = false
		var isPrivileged = false  // this should always be false because the sidecar does not require privileges on the node outside its kernel namespace - kb - 2025-01-16
		var sidecarSecurityContext *corev1.SecurityContext

		if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil {
			sidecarSecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add:  []corev1.Capability{"NET_ADMIN"},  // pruned net_bind because it's only required to bind privileged low ports in proxy mode which is never used by the sidecar - kb - 2025-01-16
					Drop: []corev1.Capability{"ALL"},
				},
				RunAsUser:  &rootUser,
				Privileged: &isPrivileged,
			}
		}

		// Set container args based on log verbosity
		containerArgs := []string{"tproxy"}
		if klog.V(4).Enabled() {
			containerArgs = append(containerArgs, "--verbose")
		}

		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:            identityName,
			Image:           fmt.Sprintf("%s:%s", sidecarImage, sidecarImageVersion),
			Args:            containerArgs,
			Env:            containerEnv,
			VolumeMounts:    []corev1.VolumeMount{{Name: volumeMountName, MountPath: sidecarIdentityDir}},
			SecurityContext: sidecarSecurityContext,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceEphemeralStorage: resource.MustParse("10Mi"),
				},
			},
		})

		containersBytes, err := json.Marshal(&pod.Spec.Containers)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
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
			{
				OP:    "replace",
				Path:  "/metadata/annotations",
				Value: annotationsBytes,
			},
		}

		// update Pod Security Context RunAsNonRoot to false
		if podSecurityOverride {
			pod.Spec.SecurityContext.RunAsNonRoot = &isNotTrue
			podSecurityContextBytes, err = json.Marshal(&pod.Spec.SecurityContext)
			if err != nil {
				klog.Error(err)
				return failureResponse(reviewResponse, err)
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
			return failureResponse(reviewResponse, err)
		}

		reviewResponse.Patch = patchBytes
		// klog.Infof(fmt.Sprintf("Patch bytes: %s", reviewResponse.Patch))
		pt := admissionv1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt

	case "DELETE":
		pod := corev1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		identityName, err := getZitiIdentityName(pod.Annotations)
		if err != nil {
			// Don't block deletion if we can't get the identity name
			klog.V(4).Infof("%s: %v", warnIdentityNameExtract, err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, warnIdentityNameExtract)
		}
		klog.V(4).Infof("ziti identity name from annotations is %s", identityName)

		zec, err := ze.Client(&zecfg)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		zId, found, err := findIdentity(identityName, zec)
		if err != nil {
			// Don't block deletion if we can't find the identity
			klog.V(4).Infof("%s: %v", warnIdentityListFailed, err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, warnIdentityListFailed)
		}
		if found {
			err = ze.DeleteIdentity(zId, zec)
			if err != nil {
				klog.Errorf("failed to delete ziti identity '%s': %v", identityName, err)
				reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
			} else {
				klog.Infof("deleted ziti identity '%s'", identityName)
			}
		} else {
			klog.V(4).Infof("ziti identity '%s' does not exist", identityName)
		}

	case "UPDATE":
		klog.V(4).Infof("Starting webhook operation: %s", ar.Request.Operation)
		klog.V(5).Infof("Object: %s", ar.Request.Object.Raw)
		klog.V(5).Infof("OldObject: %s", ar.Request.OldObject.Raw)
		if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &pod); err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, &oldPod); err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}

		identityName, err := getZitiIdentityName(pod.Annotations)
		if err != nil {
			klog.V(4).Infof("%s: %v", warnIdentityNameExtract, err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, warnIdentityNameExtract)
		} else {
			klog.V(4).Infof("ziti identity name from annotations is %s", identityName)
		}

		var roles []string
		klog.V(4).Infof("pod Annotations are %v", pod.Annotations)
		newRoles, newOk := getIdentityAttributes(pod.Annotations)
		klog.V(4).Infof("OldPod Annotations are %v", oldPod.Annotations)
		oldRoles, oldOk := getIdentityAttributes(oldPod.Annotations)

		if !newOk && oldOk {
			// zitiRoleKey roles annotation was removed, use default app role
			roles = []string{pod.Labels["app"]}
		} else if newOk && !reflect.DeepEqual(newRoles, oldRoles) {
			// zitiRoleKey roles annotation was created or updated
			roles = newRoles
		} else {
			// no change to roles, leave empty to skip role update in admission response
		}

		klog.V(4).Infof("Roles are %v", roles)
		klog.V(4).Infof("Roles length is %d", len(roles))

		zec, err := ze.Client(&zecfg)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		zId, found, err := findIdentity(identityName, zec)
		if err != nil {
			klog.Error(err)
			reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
		}
		if found {
			identityDetails, err := ze.PatchIdentity(zId, roles, zec)
			if err != nil {
				klog.Error(err)
				reviewResponse.Warnings = append(reviewResponse.Warnings, err.Error())
			} else {
				detailsJSON, err := json.Marshal(identityDetails)
				if err != nil {
					klog.Warningf("failed to marshal identity details to JSON: %v", err)
				} else {
					klog.V(5).Infof("Updated identity details:\n%s", string(detailsJSON))
				}
			}
		} else {
			klog.Errorf("Identity '%s' not found during pod update operation", identityName)
		}
	}
	return successResponse(reviewResponse)
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
func findIdentity(name string, zec *rest_management_api_client.ZitiEdgeManagement) (zId string, ok bool, err error) {
	identityDetails, err := ze.GetIdentityByName(name, zec)
	if err != nil {
		klog.Error(err)
		return "", false, err
	}
	data := identityDetails.GetPayload().Data
	if len(data) > 1 {
		klog.Warningf("Multiple identities found with name %s. Using first match.", name)
	}
	zId = *data[0].ID
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

func getZitiIdentityName(annotations map[string]string) (string, error) {
	if name, exists := annotations[annotationIdentityName]; exists && name != "" {
		return name, nil
	}
	return "", errors.Errorf("identity name annotation '%s' not found", annotationIdentityName)
}

func buildZitiIdentityName(prefix string, pod *corev1.Pod, uid types.UID) (string, error) {
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
				klog.V(4).Infof("Setting pod name from label %s=%s", label, name)
				break
			} else {
				klog.V(4).Infof("Label %s not found", label)
			}
		}

		// Check pod name if no label was found
		if name == "" && pod.Name != "" {
			name = pod.Name
		}

		// Fall back to pod UID if nothing else is available
		if name == "" {
			return "", fmt.Errorf("failed to build identity name")
		}
	}

	// Trim the name if it's too long and not a UID
	if !isUID {
		name = trimString(name)
	}

	// Build the full identity name
	identityName := fmt.Sprintf("%s-%s-%s-%s", prefix, name, pod.Namespace, uid)

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
