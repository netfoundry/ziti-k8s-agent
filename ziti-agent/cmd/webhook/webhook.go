package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common"
	k "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/kubernetes"
	ze "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"
	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
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

// // admitv1beta1Func handles a v1beta1 admission
// type admitv1beta1Func func(admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse

// admitv1Func handles a v1 admission
type admitv1Func func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// admitHandler is a handler, for both validators and mutators, that supports multiple admission review versions
type admitHandler struct {
	// admissionv1beta1 admitv1beta1Func
	admissionv1 admitv1Func
}

func newDelegateToV1AdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		// admissionv1beta1: delegateV1beta1AdmitToV1(f),
		admissionv1: f,
	}
}

// func delegateV1beta1AdmitToV1(f admitv1Func) admitv1beta1Func {
// 	return func(review admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
// 		in := admissionv1.AdmissionReview{Request: convertAdmissionRequestToV1(review.Request)}
// 		out := f(in)
// 		return convertAdmissionResponseToV1beta1(out)
// 	}
// }

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
	// case admissionv1beta1.SchemeGroupVersion.WithKind("AdmissionReview"):
	// 	requestedAdmissionReview, ok := obj.(*admissionv1beta1.AdmissionReview)
	// 	if !ok {
	// 		klog.Errorf("Expected v1beta1.AdmissionReview but got: %T", obj)
	// 		return
	// 	}
	// 	responseAdmissionReview := &admissionv1beta1.AdmissionReview{}
	// 	responseAdmissionReview.SetGroupVersionKind(*gvk)
	// 	responseAdmissionReview.Response = admit.admissionv1beta1(*requestedAdmissionReview)
	// 	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
	// 	responseObj = responseAdmissionReview

	// 	klog.Infof("Admission Response UID: %s", responseAdmissionReview.Response.UID)

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

		klog.Infof("Admission Response UID: %s", responseAdmissionReview.Response.UID)

	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func zitiClientImpl() *rest_management_api_client.ZitiEdgeManagement {
	// initialize ziti client
	tlsCertificate, _ := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	if err != nil {
		klog.Error(err)
	}
	parsedCert, err := x509.ParseCertificate(tlsCertificate.Certificate[0])
	if err != nil {
		klog.Error(err)
	}
	cfg := ze.Config{ApiEndpoint: zitiCtrlMgmtApi,
		Cert:       parsedCert,
		PrivateKey: tlsCertificate.PrivateKey}
	zc, err := ze.Client(&cfg)
	if err != nil {
		klog.Error(err)
	}
	return zc
}

func serveZitiTunnel(w http.ResponseWriter, r *http.Request) {
	zh := &zitiHandler{
		KC: &clusterClient{Client: k.Client()},
		ZC: &zitiClient{Client: zitiClientImpl()},
		Config: &ZitiConfig{
			VolumeMountName: "sidecar-ziti-identity",
			LabelKey:        "openziti/tunnel-inject",
			RoleKey:         zitiRoleKey,
			Image:           sidecarImage,
			ImageVersion:    sidecarImageVersion,
			Prefix:          sidecarPrefix,
			labelDelValue:   "disable",
			labelCrValue:    "enable",
			resolverIp:      clusterDnsServiceIP,
		}}
	serve(w, r, newDelegateToV1AdmitHandler(zh.HandleAdmissionRequest))
}

func serveZitiRouter(w http.ResponseWriter, r *http.Request) {
	zh := &zitiHandler{}
	serve(w, r, newDelegateToV1AdmitHandler(zh.HandleAdmissionRequest))
}

func webhook(cmd *cobra.Command, args []string) {

	klog.Infof("Current version is %s", common.Version)
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

	// process ziti admin user certs passed from the file through the command line
	if zitiCtrlClientCertFile != "" && zitiCtrlClientKeyFile != "" {
		zitiAdminCert, err = os.ReadFile(zitiCtrlClientCertFile)
		if err != nil {
			klog.Info(err)
		}

		zitiAdminKey, err = os.ReadFile(zitiCtrlClientKeyFile)
		if err != nil {
			klog.Info(err)
		}
	}

	// load env vars to override the command line vars if any
	lookupEnvVars()

	klog.Infof("AC WH Server is listening on port %d", port)
	http.HandleFunc("/ziti-tunnel", serveZitiTunnel)
	http.HandleFunc("/ziti-router", serveZitiRouter)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(cert, key),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}
