package webhook

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

type MissingEnvVarError struct {
	variable string
}

type MissingCmdLineVarError struct {
	variable string
}

func configTLS(cert, key []byte) *tls.Config {
	sCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		klog.Fatalf("Failed to load the webhook server's x509 cert %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func (e *MissingEnvVarError) Error() string {
	return fmt.Sprintf("Missing environment variable: %s", e.variable)
}

func (e *MissingCmdLineVarError) Error() string {
	return fmt.Sprintf("Missing commandline variable: %s", e.variable)
}

func lookupEnvVars() {
	// Environmental Variables to override the commandline inputs

	value, ok := os.LookupEnv("TLS_CERT")
	if ok && len(value) > 0 {
		cert = []byte(value)
	}
	if len(cert) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "TLS_CERT"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "TLS_CERT"})
	}

	value, ok = os.LookupEnv("TLS_PRIVATE_KEY")
	if ok && len(value) > 0 {
		key = []byte(value)
	}
	if len(key) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "TLS_PRIVATE_KEY"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "TLS_PRIVATE_KEY"})
	}

	value, ok = os.LookupEnv("PORT")
	if ok && len(value) > 0 {
		var err error
		port, err = strconv.Atoi(value)
		if err != nil {
			klog.Fatal(err)
		}
	}
	if port == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "PORT"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "PORT"})
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE")
	if ok && len(value) > 0 {
		sidecarImage = value
	}
	if len(sidecarImage) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "SIDECAR_IMAGE"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "SIDECAR_IMAGE"})
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE_VERSION")
	if ok && len(value) > 0 {
		sidecarImageVersion = value
	}
	if len(sidecarImageVersion) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "SIDECAR_IMAGE_VERSION"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "SIDECAR_IMAGE_VERSION"})
	}

	value, ok = os.LookupEnv("SIDECAR_PREFIX")
	if ok && len(value) > 0 {
		sidecarPrefix = value
	}
	if len(sidecarPrefix) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "SIDECAR_PREFIX"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "SIDECAR_PREFIX"})
		klog.Fatal("sidecarPrefix cannot be empty")
	} else {
		klog.V(4).Infof("sidecarPrefix: %s", sidecarPrefix)
	}

	value, ok = os.LookupEnv("ZITI_MGMT_API")
	if ok && len(value) > 0 {
		zitiCtrlMgmtApi = value
	}
	if len(zitiCtrlMgmtApi) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "ZITI_MGMT_API"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "ZITI_MGMT_API"})
	}

	value, ok = os.LookupEnv("ZITI_ADMIN_CERT")
	if ok && len(value) > 0 {
		zitiAdminCert = []byte(value)
	}
	if zitiAdminCert == nil {
		klog.V(4).Info(&MissingEnvVarError{variable: "ZITI_ADMIN_CERT"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "ZITI_ADMIN_CERT"})
	}

	value, ok = os.LookupEnv("ZITI_ADMIN_KEY")
	if ok && len(value) > 0 {
		zitiAdminKey = []byte(value)
	}
	if zitiAdminKey == nil {
		klog.V(4).Info(&MissingEnvVarError{variable: "ZITI_ADMIN_KEY"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "ZITI_ADMIN_KEY"})
	}

	value, ok = os.LookupEnv("ZITI_CTRL_CA_BUNDLE")
	if ok && len(value) > 0 {
		zitiCtrlCaBundle = []byte(value)
		klog.V(5).Infof("CA bundle content from env: %s", string(zitiCtrlCaBundle))
	}
	if zitiCtrlCaBundle == nil {
		klog.V(4).Info(&MissingEnvVarError{variable: "ZITI_CTRL_CA_BUNDLE"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "ZITI_CTRL_CA_BUNDLE"})
	}

	value, ok = os.LookupEnv("POD_SECURITY_CONTEXT_OVERRIDE")
	if ok && len(value) > 0 {
		var err error
		podSecurityOverride, err = strconv.ParseBool(value)
		if err != nil {
			klog.Info(err)
		}
	}

	value, ok = os.LookupEnv("SEARCH_DOMAINS")
	if ok && len(value) > 0 {
		searchDomains = strings.Split(value, ",")
	}
	if len(searchDomains) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "SEARCH_DOMAINS"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "SEARCH_DOMAINS"})
		klog.Info("Custom DNS search domains not set, using Kubernetes defaults")
	} else {
		klog.Infof("Custom DNS search domains: %s", searchDomains)
	}

	value, ok = os.LookupEnv("ZITI_ROLE_KEY")
	if ok && len(value) > 0 {
		zitiRoleKey = value
	}
	if len(zitiRoleKey) == 0 {
		klog.V(4).Info(&MissingEnvVarError{variable: "ZITI_ROLE_KEY"})
		klog.V(4).Info(&MissingCmdLineVarError{variable: "ZITI_ROLE_KEY"})
	}
}
