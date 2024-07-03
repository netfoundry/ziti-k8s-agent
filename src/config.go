package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// Define sidecar config options
type SidecarConfig struct {
	Image string `json:"image"`
	Name  string `json:"name"`
}

type MissingEnvVarError struct {
	variable string
}

func configTLS(cert, key []byte) *tls.Config {
	sCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		klog.Fatalf("Failed to load the 509x certs %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

func (e *MissingEnvVarError) Error() string {
	return fmt.Sprintf("Missing environment variable: %s", e.variable)
}

func lookupEnvVars() {
	// Environmental Variables to override the commandline inputs
	value, ok = os.LookupEnv("TLS-CERT")
	if !ok || len(value) == 0 {
		klog.Error(&MissingEnvVarError{variable: "TLS-CERT"})
	} else {
		cert = []byte(value)
	}

	value, ok = os.LookupEnv("TLS-PRIVATE-KEY")
	if !ok || len(value) == 0 {
		klog.Error(&MissingEnvVarError{variable: "TLS-PRIVATE-KEY"})
	} else {
		key = []byte(value)
	}

	value, ok = os.LookupEnv("PORT")
	if (!ok || len(value) == 0) && port <= 0 {
		klog.Error(&MissingEnvVarError{variable: "PORT"})
	} else {
		port, _ = strconv.Atoi(value)
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE")
	if (!ok || len(value) == 0) && len(sidecarImage) == 0 {
		klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE"})
	} else {
		sidecarImage = value
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE_VERSION")
	if (!ok || len(value) == 0) && len(sidecarImageVersion) == 0 {
		klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE_VERSION"})
	} else {
		sidecarImageVersion = value
	}

	value, ok = os.LookupEnv("SIDECAR_PREFIX")
	if (!ok || len(value) == 0) && len(sidecarPrefix) == 0 {
		klog.Error(&MissingEnvVarError{variable: "SIDECAR_PREFIX"})
	} else {
		sidecarPrefix = value
	}

	value, ok = os.LookupEnv("ZITI_CTRL_MGMT_API")
	if !ok || len(value) == 0 {
		klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_MGMT_API"})
	} else {
		zitiCtrlMgmtApi = value
	}

	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_CERT")
	if !ok || len(value) == 0 {
		klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_CERT"})
	} else {
		zitiAdminCert = []byte(value)
	}

	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_KEY")
	if !ok || len(value) == 0 {
		klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_KEY"})
	} else {
		zitiAdminKey = []byte(value)
	}

	value, ok = os.LookupEnv("POD_SECURITY_CONTEXT_OVERRIDE")
	if ok {
		var err error
		podSecurityOverride, err = strconv.ParseBool(value)
		if err != nil {
			klog.Info(err)
		}
	}
	value, ok = os.LookupEnv("CLUSTER_DNS_SVC_IP")
	if !ok || len(value) == 0 {
		klog.Infof(fmt.Sprintf("Custom DNS Server IP is not set"))
		klog.Infof(fmt.Sprintf("DNS Service ClusterIP will be looked up"))
	} else {
		clusterDnsServiceIP = value
	}

	value, ok = os.LookupEnv("ZITI_ROLE_KEY")
	if !ok || len(value) == 0 {
		klog.Infof(fmt.Sprintf("A ziti role key is not present in the pod annotations"))
	} else {
		zitiRoleKey = value
	}

	value, ok = os.LookupEnv("SEARCH_DOMAIN_LIST")
	if ok {
		searchDomainList = []string(strings.Split(value, " "))
	} else {
		klog.Infof(fmt.Sprintf("A list of Custom DNS search domains for host-name lookup is not set, will set the Kubernetes default"))
	}

}
