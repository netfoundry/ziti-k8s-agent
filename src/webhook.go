package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

var (
	certFile               string
	keyFile                string
	cert                   []byte
	key                    []byte
	zitiAdminCert          []byte
	zitiAdminKey           []byte
	port                   int
	sidecarImage           string
	sidecarImageVersion    string
	sidecarPrefix          string
	zitiCtrlAddress        string
	zitiCtrlClientCertFile string
	zitiCtrlClientKeyFile  string
	podSecurityOverride    bool
	clusterDnsServiceIP    string
	searchDomainList       []string
	zitiIdentityRoles      []string
	zitiRoleKey            string
	value                  string
	ok                     bool
	err                    error
	runtimeScheme          = runtime.NewScheme()
)

var CmdWebhook = &cobra.Command{
	Use:   "webhook",
	Short: "Starts a HTTP server,  Mutating Admission Webhook for injecting ziti sidecars",
	Long: `Starts a HTTP server, Mutating Admission Webhook for injecting ziti sidecars.
After deploying it to Kubernetes cluster, it listens to events related to pods CRUD operations.`,
	Args: cobra.MaximumNArgs(0),
	Run:  webhook,
}

func init() {
	CmdWebhook.Flags().StringVar(&certFile, "tls-cert-file", "",
		"File containing the default x509 Certificate for HTTPS.")
	CmdWebhook.Flags().StringVar(&keyFile, "tls-private-key-file", "",
		"File containing the default x509 private key matching --tls-cert-file.")
	CmdWebhook.Flags().IntVar(&port, "port", 9443,
		"Secure port that the webhook listens on")
	CmdWebhook.Flags().StringVar(&sidecarImage, "sidecar-image", "openziti/ziti-tunnel",
		"Image to be used as the injected sidecar")
	CmdWebhook.Flags().StringVar(&sidecarImageVersion, "sidecar-image-version", "latest",
		"Image Varsion to be used as the injected sidecar")
	CmdWebhook.Flags().StringVar(&sidecarPrefix, "sidecar-prefix", "zt",
		"ContainerName to be used for the injected sidecar")
	CmdWebhook.Flags().StringVar(&zitiCtrlAddress, "ziti-ctrl-addr", "",
		"Ziti Controller IP Address / FQDN")
	CmdWebhook.Flags().StringVar(&zitiCtrlClientCertFile, "ziti-ctrl-client-cert-file", "",
		"Ziti Controller Client Certificate")
	CmdWebhook.Flags().StringVar(&zitiCtrlClientKeyFile, "ziti-ctrl-client-key-file", "",
		"Ziti Controller Client Private Key")
	CmdWebhook.Flags().BoolVar(&podSecurityOverride, "pod-sc-override", false,
		"Override the security context at pod level, i.e. runAsNonRoot: false")
	CmdWebhook.Flags().StringVar(&clusterDnsServiceIP, "cluster-dns-svc-ip", "",
		"Cluster DNS Service IP")
	CmdWebhook.Flags().StringSliceVar(&searchDomainList, "search-domain-list", []string{},
		"A list of DNS search domains for host-name lookup")
	CmdWebhook.Flags().StringVar(&zitiRoleKey, "ziti-role-key", "",
		"Ziti Identity Role Key used in pod annotation")

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

		klog.Infof(fmt.Sprintf("Admission Response v1: %s", responseObj))

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

		klog.Infof(fmt.Sprintf("Admission Response UID: %s", responseAdmissionReview.Response.UID))

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

func serveZitiTunnelSC(w http.ResponseWriter, r *http.Request) {
	serve(w, r, newDelegateToV1AdmitHandler(zitiTunnel))
}

func webhook(cmd *cobra.Command, args []string) {

	klog.Infof("Current version is %s", Version)

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

	http.HandleFunc("/ziti-tunnel", serveZitiTunnelSC)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(cert, key),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}
