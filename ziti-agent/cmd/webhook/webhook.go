package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type admitv1Func func(context.Context, admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

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
			klog.V(3).Infof("Request ID %s processed in %s", requestedAdmissionReview.Request.UID, duration.Round(time.Millisecond))
		}()

		responseAdmissionReview := &admissionv1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.admissionv1(context.Background(), *requestedAdmissionReview)
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

	cfg := zitiedge.Config{ApiEndpoint: runtimeConfig.Controller.MgmtAPI,
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
			ZitiType:             zitiTypeTunnel,
			VolumeMountName:      runtimeConfig.Sidecar.VolumeMountName,
			LabelKey:             "tunnel.openziti.io/enabled",
			RoleKey:              runtimeConfig.Controller.RoleKey,
			Image:                runtimeConfig.Sidecar.Image,
			ImageVersion:         runtimeConfig.Sidecar.ImageVersion,
			ImagePullPolicy:      runtimeConfig.Sidecar.ImagePullPolicy,
			IdentityDir:          runtimeConfig.Sidecar.IdentityDir,
			Prefix:               runtimeConfig.Sidecar.Prefix,
			LabelDelValue:        "false",
			LabelCrValue:         "true",
			ResolverIp:           runtimeConfig.Sidecar.ResolverIP,
			DnsUpstreamEnabled:   runtimeConfig.Sidecar.DnsUpstreamEnabled,
			Unanswerable:         runtimeConfig.Sidecar.DnsUnanswerable,
			SearchDomains:        runtimeConfig.Sidecar.SearchDomains,
			PodSecurityOverride:  runtimeConfig.Security.PodSecurityContextOverride,
			RouterConfig:         routerConfig{},
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
			ZitiType:            zitiTypeRouter,
			LabelKey:            "router.openziti.io/enabled",
			AnnotationKey:       "openziti/router-name",
			LabelDelValue:       "false",
			LabelCrValue:        "true",
			Prefix:              runtimeConfig.Sidecar.Prefix,
			ResolverIp:          runtimeConfig.Sidecar.ResolverIP,
			PodSecurityOverride: runtimeConfig.Security.PodSecurityContextOverride,
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

	var err error
	runtimeConfig, err = loadConfig(configPath)
	if err != nil {
		klog.Fatalf("failed to load configuration: %v", err)
	}

	loadCertificatesFromEnv()
	
	klog.Infof("Running version is %s", common.Version)

	if len(cert) == 0 || len(key) == 0 {
		klog.Fatal("TLS_CERT and TLS_PRIVATE_KEY must be provided via environment variables")
	}

	if len(zitiAdminCert) == 0 || len(zitiAdminKey) == 0 || len(zitiCtrlCaBundle) == 0 {
		klog.Fatal("ZITI_ADMIN_CERT, ZITI_ADMIN_KEY, and ZITI_CTRL_CA_BUNDLE must be provided via environment variables")
	}

	port := runtimeConfig.Server.Port
	http.HandleFunc("/ziti-tunnel", serveZitiTunnel)
	http.HandleFunc("/ziti-router", serveZitiRouter)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(cert, key),
	}
	if err = server.ListenAndServeTLS("", ""); err != nil {
		klog.Fatal(err)
	}
	klog.Infof("ziti agent webhook server is listening on port %d", port)
}
