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
	if ok {
		cert = []byte(value)
	} else {
		if cert == nil {
			klog.Error(&MissingEnvVarError{variable: "TLS-CERT"})
		}
	}
	value, ok = os.LookupEnv("TLS-PRIVATE-KEY")
	if ok {
		key = []byte(value)
	} else {
		if key == nil {
			klog.Error(&MissingEnvVarError{variable: "TLS-PRIVATE-KEY"})
		}
	}
	value, ok = os.LookupEnv("PORT")
	if ok {
		port, _ = strconv.Atoi(value)
	} else {
		if port <= 0 {
			klog.Error(&MissingEnvVarError{variable: "PORT"})
		}
	}
	value, ok = os.LookupEnv("SIDECAR_IMAGE")
	if ok {
		sidecarImage = value
	} else {
		if len(sidecarImage) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE"})
		}
	}
	value, ok = os.LookupEnv("SIDECAR_IMAGE_VERSION")
	if ok {
		sidecarImageVersion = value
	} else {
		if len(sidecarImageVersion) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE_VERSION"})
		}
	}
	value, ok = os.LookupEnv("SIDECAR_PREFIX")
	if ok {
		sidecarPrefix = value
	} else {
		if len(sidecarPrefix) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_PREFIX"})
		}
	}
	value, ok = os.LookupEnv("ZITI_CTRL_MGMT_API")
	if ok {
		zitiCtrlMgmtApi = value
	} else {
		if len(zitiCtrlMgmtApi) == 0 {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_MGMT_API"})
		}
	}
	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_CERT")
	if ok {
		zitiAdminCert = []byte(value)
	} else {
		if zitiAdminCert == nil {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_CERT"})
		}
	}
	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_KEY")
	if ok {
		zitiAdminKey = []byte(value)
	} else {
		if zitiAdminKey == nil {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_KEY"})
		}
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
	if ok {
		clusterDnsServiceIP = value
	} else {
		if len(clusterDnsServiceIP) == 0 {
			klog.Infof(fmt.Sprintf("Custom DNS Server IP is not set"))
			klog.Infof(fmt.Sprintf("DNS Service ClusterIP will be looked up"))
		}
	}
	value, ok = os.LookupEnv("SEARCH_DOMAIN_LIST")
	if ok {
		searchDomainList = []string(strings.Split(value, " "))
	} else {
		klog.Infof(fmt.Sprintf("A list of DNS search domains for host-name lookup is not set"))
	}
	value, ok = os.LookupEnv("ZITI_ROLE_KEY")
	if ok {
		zitiRoleKey = value
	} else {
		if len(zitiRoleKey) == 0 {
			klog.Infof(fmt.Sprintf("A ziti role key is not present in the pod annotations"))
		}
	}
}
