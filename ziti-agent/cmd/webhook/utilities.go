package webhook

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func buildZitiIdentityName(prefix string, podMeta *metav1.ObjectMeta, uid types.UID) (string, error) {
	var name string

	// Check for explicit annotation first
	if annotatedName, exists := podMeta.Annotations[annotationIdentityName]; exists && annotatedName != "" {
		name = annotatedName
	} else {
		// Check labels in order of precedence
		labels := []string{labelApp, labelAppName, labelAppInstance, labelAppComponent}
		for _, label := range labels {
			if labelName, exists := podMeta.Labels[label]; exists && labelName != "" {
				name = labelName
				break
			}
		}
	}

	if name == "" {
		return "", errors.New("failed to build identity name: no valid name found in annotations or labels")
	}

	// Build base name with prefix, name and namespace
	baseName := fmt.Sprintf("%s-%s-%s", prefix, name, podMeta.Namespace)

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
