package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common"
	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

func init() {
	/*
		AdmissionReview is registered for version admission.k8s.io/v1 or admission.k8s.io/v1beta1
		in scheme "https://github.com/kubernetes/apimachinery/blob/master/pkg/runtime/scheme.go:100"
	*/
	addToScheme(scheme)
}

// admitv1beta1Func handles a v1beta1 admission
type admitv1beta1Func func(admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse

// admitv1Func handles a v1 admission
type admitv1Func func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// admitHandler is a handler, for both validators and mutators, that supports multiple admission review versions
type admitHandler struct {
	admissionv1beta1 admitv1beta1Func
	admissionv1      admitv1Func
}

func newDelegateToV1AdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		admissionv1beta1: delegateV1beta1AdmitToV1(f),
		admissionv1:      f,
	}
}

func delegateV1beta1AdmitToV1(f admitv1Func) admitv1beta1Func {
	return func(review admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
		in := admissionv1.AdmissionReview{Request: convertAdmissionRequestToV1(review.Request)}
		out := f(in)
		return convertAdmissionResponseToV1beta1(out)
	}
}

// serve handles the http portion of a request prior to handing to an admit function
func serve(w http.ResponseWriter, r *http.Request, admit admitHandler) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case admissionv1beta1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*admissionv1beta1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1beta1.AdmissionReview but got: %T", obj)
			return
		}
		responseAdmissionReview := &admissionv1beta1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.admissionv1beta1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
		responseJSON, err := json.Marshal(responseAdmissionReview)
		if err != nil {
			klog.Warningf("failed to marshal review response to JSON: %v", err)
		} else {
			klog.V(5).Infof("Review response:\n%s", string(responseJSON))
		}

	case admissionv1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*admissionv1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1.AdmissionReview but got: %T", obj)
			return
		}
		responseAdmissionReview := &admissionv1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.admissionv1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
		responseJSON, err := json.Marshal(responseAdmissionReview)
		if err != nil {
			klog.Warningf("failed to marshal review response to JSON: %v", err)
		} else {
			klog.V(5).Infof("Review response:\n%s", string(responseJSON))
		}

	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	responseBytes, err := json.Marshal(responseObj)
	if err != nil {
		err = fmt.Errorf("failed to marshal review response to JSON: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		klog.V(5).Infof("Review response:\n%s", string(responseBytes))
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(responseBytes); err != nil {
		err = fmt.Errorf("failed to write response: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func serveZitiTunnelSC(w http.ResponseWriter, r *http.Request) {
	serve(w, r, newDelegateToV1AdmitHandler(handleZitiTunnelAdmission))
}

func webhook(cmd *cobra.Command, args []string) {

	// load env vars to override the command line vars if any
	lookupEnvVars()

	klog.Infof("Running version is %s", common.Version)

	// process certs passed from the file through the command line
	if certFile != "" && keyFile != "" {
		cert, err = os.ReadFile(certFile)
		if err != nil {
			klog.Info(err)
		}

		key, err = os.ReadFile(keyFile)
		if err != nil {
			klog.Info(err)
		}
	}
	if cert == nil || key == nil {
		klog.Fatal("Cert and key required, but one or both are missing")
	}

	// process ziti admin user identity passed as separate file paths instead of env vars
	if zitiCtrlClientCertFile != "" && zitiCtrlClientKeyFile != "" && zitiCtrlCaBundleFile != "" {
		zitiAdminCert, err = os.ReadFile(zitiCtrlClientCertFile)
		if err != nil {
			klog.Info(err)
		}

		zitiAdminKey, err = os.ReadFile(zitiCtrlClientKeyFile)
		if err != nil {
			klog.Info(err)
		}

		zitiCtrlCaBundle, err = os.ReadFile(zitiCtrlCaBundleFile)
		if err != nil {
			klog.Info(err)
		}
	}

	if zitiAdminCert == nil || zitiAdminKey == nil || zitiCtrlCaBundle == nil {
		klog.Fatal("ziti admin cert, key, and root ca bundle are required as env var or run parameter, but at least one is missing")
	}

	http.HandleFunc("/ziti-tunnel", serveZitiTunnelSC)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(cert, key),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		klog.Fatal(err)
	}
	klog.Infof("ziti agent webhook server is listening on port %d", port)
}
