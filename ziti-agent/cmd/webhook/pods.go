package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"reflect"

	k "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/kubernetes"
	ze "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	volumeMountName string = "sidecar-ziti-identity"
	labelTunnelKey  string = "openziti/tunnel-inject"
	labelRouterKey  string = "openziti/router-manage"
	labelDelete     string = "disable"
	labelCreate     string = "enable"
)

func zitiTunnel(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}
	labelValue := ""
	deleteAction := false
	// Get Pod and OldPod Details
	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &pod); err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, &oldPod); err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	// Initialized kubernetes client
	kc := k.Client()
	// Get Namespaces
	namespaces, err := kc.CoreV1().Namespaces().List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	// Read namespace label value to look for delete action
	for _, namespace := range namespaces.Items {
		if namespace.Name == pod.Namespace || namespace.Name == oldPod.Namespace {
			if v, ok := namespace.Labels[labelTunnelKey]; ok {
				switch v {
				case labelCreate:
					deleteAction = false
				case labelDelete:
					deleteAction = true
				default:
					klog.Infof("Namespace Label Value %v does not matched the expected tunnel-inject", labelValue)
					return successResponse(reviewResponse)
				}
			}
		}
	}

	klog.Infof("Admission Request UID: %s", ar.Request.UID)
	klog.Infof("Admission Request Operation: %s", ar.Request.Operation)
	klog.Infof("Namespace label Value: %v", labelValue)
	klog.Infof("Deletion Action: %v", deleteAction)

	// parse ziti admin certs and initilaized ziti ctrl config
	zitiTlsCertificate, _ := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	parsedCert, err := x509.ParseCertificate(zitiTlsCertificate.Certificate[0])
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
	zecfg := ze.Config{ApiEndpoint: zitiCtrlMgmtApi, Cert: parsedCert, PrivateKey: zitiTlsCertificate.PrivateKey}

	// Switch between Admission Operations
	switch {

	case ar.Request.Operation == "CREATE" && !deleteAction:

		roles, ok := ze.GetIdentityAttributes(pod.Annotations, zitiRoleKey)
		if !ok {
			roles = []string{pod.Labels["app"]}
		}

		zec, err := ze.Client(&zecfg)
		if err != nil {
			return failureResponse(reviewResponse, err)
		}

		identityToken, sidecarIdentityName, err := ze.GetIdentityToken(
			pod.Labels["app"],
			sidecarPrefix,
			ar.Request.UID, roles, zec)
		if identityToken == "" {
			return failureResponse(reviewResponse, err)
		}

		if len(clusterDnsServiceIP) == 0 {
			dnsService, err := kc.CoreV1().Services("kube-system").Get(context.TODO(), "kube-dns", metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
			}
			if len(dnsService.Spec.ClusterIP) != 0 {
				clusterDnsServiceIP = dnsService.Spec.ClusterIP
				klog.Infof("Looked up DNS SVC ClusterIP and is %s", dnsService.Spec.ClusterIP)
			} else {
				klog.Info("Looked up DNS SVC ClusterIP and is not found")
			}
		}

		// add sidecar volume to pod
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
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
			Name:            sidecarIdentityName,
			Image:           fmt.Sprintf("%s:%s", sidecarImage, sidecarImageVersion),
			Args:            []string{"tproxy", "-i", fmt.Sprintf("%v.json", sidecarIdentityName)},
			Env:             []corev1.EnvVar{{Name: "ZITI_ENROLL_TOKEN", Value: identityToken}, {Name: "NF_REG_NAME", Value: sidecarIdentityName}},
			VolumeMounts:    []corev1.VolumeMount{{Name: volumeMountName, MountPath: "/netfoundry", ReadOnly: false}},
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

	case ar.Request.Operation == "DELETE":

		zName, ok := hasContainer(oldPod.Spec.Containers, fmt.Sprintf("%s-%s", oldPod.Labels["app"], sidecarPrefix))
		if ok {
			// secretData, err := kc.CoreV1().Secrets(oldPod.Namespace).Get(context.TODO(), zName, metav1.GetOptions{})
			// if err != nil {
			// 	klog.Error(err)
			// }
			// if len(secretData.Name) > 0 {
			// 	err = kc.CoreV1().Secrets(oldPod.Namespace).Delete(context.TODO(), zName, metav1.DeleteOptions{})
			// 	if err != nil {
			// 		klog.Error(err)
			// 	} else {
			// 		klog.Infof("Secret %s was deleted at %s", zName, secretData.DeletionTimestamp)
			// 	}
			// }

			zec, err := ze.Client(&zecfg)
			if err != nil {
				return failureResponse(reviewResponse, err)
			}

			zId, ok, err := ze.FindIdentity(zName, zec)
			if err != nil {
				return failureResponse(reviewResponse, err)
			}
			if ok {
				err = ze.DeleteIdentity(zId, zec)
				if err != nil {
					return failureResponse(reviewResponse, err)
				}
			}
		}

	case ar.Request.Operation == "UPDATE" && !deleteAction:

		zName, ok := hasContainer(pod.Spec.Containers, fmt.Sprintf("%s-%s", pod.Labels["app"], sidecarPrefix))
		if ok {
			var roles []string
			klog.Infof("Pod Annotations are %s", pod.Annotations)
			newRoles, newOk := ze.GetIdentityAttributes(pod.Annotations, zitiRoleKey)
			klog.Infof("OldPod Annotations are %s", oldPod.Annotations)
			oldRoles, oldOk := ze.GetIdentityAttributes(oldPod.Annotations, zitiRoleKey)

			if !newOk && oldOk {
				// Ziti Annotation is removed
				roles = []string{pod.Labels["app"]}
			} else if newOk && !reflect.DeepEqual(newRoles, oldRoles) {
				//Ziti Annotation is created or updated
				roles = newRoles
			} else {
				roles = []string{}
			}

			klog.Infof("Roles are %s", roles)
			klog.Infof("Roles length is %d", len(roles))
			// Update only if Ziti Annotation is changed
			if len(roles) > 0 {
				zec, err := ze.Client(&zecfg)
				if err != nil {
					return failureResponse(reviewResponse, err)
				}
				zId, ok, _ := ze.FindIdentity(zName, zec)
				if ok {
					identityDetails, err := ze.PatchIdentity(zId, roles, zec)
					if err != nil {
						return failureResponse(reviewResponse, err)
					}
					klog.Infof("Updated Identity Details are %v", identityDetails)
				}
			}
		}

	}

	return successResponse(reviewResponse)
}

func zitiRouter(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	return successResponse(reviewResponse)
}
