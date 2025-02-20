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

func zitiClientImpl() (*rest_management_api_client.ZitiEdgeManagement, error) {

	// parse ziti admin certs to synchronously (blocking) create a ziti identity
	zitiAdminIdentity, err := tls.X509KeyPair(zitiAdminCert, zitiAdminKey)
	if err != nil {
		return nil, err
	}

	if len(zitiAdminIdentity.Certificate) == 0 {
		err := fmt.Errorf("no certificates found in TLS key pair")
		return nil, err
	}

	parsedCert, err := x509.ParseCertificate(zitiAdminIdentity.Certificate[0])
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("Parsed client certificate - Subject: %v, Issuer: %v", parsedCert.Subject, parsedCert.Issuer)
	klog.V(4).Infof("Loading CA bundle, size: %d bytes", len(zitiCtrlCaBundle))
	klog.V(5).Infof("CA bundle content: %s", string(zitiCtrlCaBundle))

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(zitiCtrlCaBundle) {
		err := fmt.Errorf("failed to append CA certificates from PEM")
		return nil, err
	}

	cfg := zitiedge.Config{ApiEndpoint: zitiCtrlMgmtApi,
		Cert:       parsedCert,
		PrivateKey: zitiAdminIdentity.PrivateKey,
		CAS:        *certPool,
	}

	cfg.CABundle = zitiCtrlCaBundle
	zc, err := zitiedge.Client(&cfg)

	return zc, err

}

func serveZitiTunnel(w http.ResponseWriter, r *http.Request) {

	kc, err := k.Client()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		err = fmt.Errorf("failed to initilize cluster client: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zc, err := zitiClientImpl()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		err = fmt.Errorf("failed to initilize ziti client: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zh := newZitiHandler(
		&clusterClient{client: kc},
		&zitiClient{client: zc},
		&zitiConfig{
			ZitiType:        zitiTypeTunnel,
			VolumeMountName: "ziti-identity",
			LabelKey:        "tunnel.openziti.io/enabled",
			RoleKey:         zitiRoleKey,
			Image:           sidecarImage,
			ImageVersion:    sidecarImageVersion,
			ImagePullPolicy: sidecarImagePullPolicy,
			IdentityDir:     sidecarIdentityDir,
			Prefix:          sidecarPrefix,
			LabelDelValue:   "false",
			LabelCrValue:    "true",
			ResolverIp:      clusterDnsServiceIP,
			RouterConfig:    routerConfig{},
		},
	)
	serve(w, r, newAdmitHandler(zh.handleAdmissionRequest))

}

func serveZitiRouter(w http.ResponseWriter, r *http.Request) {

	kc, err := k.Client()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		err = fmt.Errorf("failed to initilize cluster client: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zc, err := zitiClientImpl()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		err = fmt.Errorf("failed to initilize ziti client: %v", err)
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zh := newZitiHandler(
		&clusterClient{client: kc},
		&zitiClient{client: zc},
		&zitiConfig{
			ZitiType:      zitiTypeRouter,
			LabelKey:      "router.openziti.io/enabled",
			AnnotationKey: "openziti/router-name",
			LabelDelValue: "false",
			LabelCrValue:  "true",
			Prefix:        sidecarPrefix,
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

	// klog.Infof("AC WH Server is listening on port %d", port)
	http.HandleFunc("/ziti-tunnel", serveZitiTunnel)
	http.HandleFunc("/ziti-router", serveZitiRouter)
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
