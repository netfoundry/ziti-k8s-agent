package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common"
	k "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/kubernetes"
	zitiedge "github.com/netfoundry/ziti-k8s-agent/ziti-agent/pkg/ziti-edge"
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

type admitv1Func func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

type admitHandler struct {
	admissionv1 admitv1Func
}

func newAdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		admissionv1: f,
	}
}

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

	case admissionv1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*admissionv1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1.AdmissionReview but got: %T", obj)
			return
		}
		// Report the time taken to process the request
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			klog.Infof("Request ID %s processed in %s", requestedAdmissionReview.Request.UID, duration.Round(time.Millisecond))
		}()

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

func zitiClientImpl() (*rest_management_api_client.ZitiEdgeManagement, error) {
	// initialize ziti client
	tlsCertificate, _ := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	if err != nil {
		return nil, err
	}

	parsedCert, err := x509.ParseCertificate(tlsCertificate.Certificate[0])
	if err != nil {
		return nil, err
	}

	cfg := zitiedge.Config{ApiEndpoint: zitiCtrlMgmtApi,
		Cert:       parsedCert,
		PrivateKey: tlsCertificate.PrivateKey}
	zc, err := zitiedge.Client(&cfg)

	return zc, err
}

func serveZitiTunnel(w http.ResponseWriter, r *http.Request) {

	client, err := zitiClientImpl()
	zh := newZitiHandler(
		&clusterClient{client: k.Client()},
		&zitiClient{client: client, err: err},
		&zitiConfig{
			ZitiType:        zitiTypeTunnel,
			VolumeMountName: "sidecar-ziti-identity",
			LabelKey:        "openziti/tunnel-inject",
			RoleKey:         zitiRoleKey,
			Image:           tunnelImage,
			ImageVersion:    tunnelImageVersion,
			Prefix:          zitiPrefix,
			LabelDelValue:   "disable",
			LabelCrValue:    "enable",
			ResolverIp:      clusterDnsServiceIP,
			RouterConfig:    routerConfig{},
		},
	)
	serve(w, r, newAdmitHandler(zh.handleAdmissionRequest))

}

func serveZitiRouter(w http.ResponseWriter, r *http.Request) {

	client, err := zitiClientImpl()
	zh := newZitiHandler(
		&clusterClient{client: k.Client()},
		&zitiClient{client: client, err: err},
		&zitiConfig{
			ZitiType:      zitiTypeRouter,
			LabelKey:      "openziti/router-manage",
			AnnotationKey: "openziti/router-name",
			LabelDelValue: "disable",
			LabelCrValue:  "enable",
			Prefix:        zitiPrefix,
			ResolverIp:    clusterDnsServiceIP,
			RouterConfig: routerConfig{
				Cost:              0,
				Disabled:          false,
				IsTunnelerEnabled: false,
				RoleAttributes:    []string{"router"},
			},
		},
	)
	serve(w, r, newAdmitHandler(zh.handleAdmissionRequest))

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

	// klog.Infof("AC WH Server is listening on port %d", port)
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
