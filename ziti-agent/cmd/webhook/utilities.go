package webhook

import (
	"fmt"
	"regexp"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type JsonPatchEntry struct {
	OP    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
	From  string      `json:"from,omitempty"`
}

func hasContainer(containers []corev1.Container, prefix string) (string, bool) {
	for _, container := range containers {
		if strings.HasPrefix(container.Name, prefix) {
			return container.Name, true
		}
	}
	return "", false
}

func filterMapValuesByKey(values map[string]string, key string) ([]string, bool) {

	value, ok := values[key]
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
//
//	ar: The admissionv1.AdmissionResponse to be updated.
//	err: The error that occurred, which will be logged and included in the response reason.
//
// Returns:
//
//	A pointer to the updated admissionv1.AdmissionResponse with Allowed set to false,
//	and the Result status set to "Failure" with a reason including the error message.
func failureResponse(ar admissionv1.AdmissionResponse, err error) *admissionv1.AdmissionResponse {
	ar.Allowed = false
	ar.Result = &metav1.Status{
		Status:  "Failure",
		Message: err.Error(),
		Reason:  metav1.StatusReason(fmt.Sprintf("Ziti Controller -  %s", err)),
	}
	return &ar
}

// successResponse sets the admission response as a success.
//
// Args:
//
//	ar: The admissionv1.AdmissionResponse to be updated.
//
// Returns:
//
//	A pointer to the updated admissionv1.AdmissionResponse with Allowed set to true,
//	and the Result status set to "Success".
func successResponse(ar admissionv1.AdmissionResponse) *admissionv1.AdmissionResponse {
	ar.Allowed = true
	ar.Result = &metav1.Status{
		Status: "Success",
	}
	return &ar
}
