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
		klog.Fatalf("Failed to load the 509x certs %v", err)
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
	value, ok = os.LookupEnv("TLS-CERT")
	if ok && len(value) > 0 {
		cert = []byte(value)
	} else {
		if cert == nil {
			klog.Error(&MissingEnvVarError{variable: "TLS-CERT"})
			klog.Error(&MissingCmdLineVarError{variable: "TLS-CERT"})
		}
	}

	value, ok = os.LookupEnv("TLS-PRIVATE-KEY")
	if ok && len(value) > 0 {
		key = []byte(value)
	} else {
		if key == nil {
			klog.Error(&MissingEnvVarError{variable: "TLS-PRIVATE-KEY"})
			klog.Error(&MissingCmdLineVarError{variable: "TLS-PRIVATE-KEY"})
		}
	}

	value, ok = os.LookupEnv("PORT")
	if ok && len(value) > 0 {
		port, _ = strconv.Atoi(value)
	} else {
		if port <= 0 {
			klog.Error(&MissingEnvVarError{variable: "PORT"})
			klog.Error(&MissingCmdLineVarError{variable: "PORT"})
		}
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE")
	if ok && len(value) > 0 {
		sidecarImage = value
	} else {
		if len(sidecarImage) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE"})
			klog.Error(&MissingCmdLineVarError{variable: "SIDECAR_IMAGE"})
		}
	}

	value, ok = os.LookupEnv("SIDECAR_IMAGE_VERSION")
	if ok && len(value) > 0 {
		sidecarImageVersion = value
	} else {
		if len(sidecarImageVersion) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_IMAGE_VERSION"})
			klog.Error(&MissingCmdLineVarError{variable: "SIDECAR_IMAGE_VERSION"})
		}
	}

	value, ok = os.LookupEnv("SIDECAR_PREFIX")
	if ok && len(value) > 0 {
		sidecarPrefix = value
	} else {
		if len(sidecarPrefix) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SIDECAR_PREFIX"})
			klog.Error(&MissingCmdLineVarError{variable: "SIDECAR_PREFIX"})
		}
	}

	value, ok = os.LookupEnv("ZITI_CTRL_MGMT_API")
	if ok && len(value) > 0 {
		zitiCtrlMgmtApi = value
	} else {
		if len(zitiCtrlMgmtApi) == 0 {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_MGMT_API"})
			klog.Error(&MissingCmdLineVarError{variable: "ZITI_CTRL_MGMT_API"})
		}
	}

	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_CERT")
	if ok && len(value) > 0 {
		zitiAdminCert = []byte(value)
	} else {
		if zitiAdminCert == nil {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_CERT"})
			klog.Error(&MissingCmdLineVarError{variable: "ZITI_CTRL_ADMIN_CERT"})
		}
	}

	value, ok = os.LookupEnv("ZITI_CTRL_ADMIN_KEY")
	if ok && len(value) > 0 {
		zitiAdminKey = []byte(value)
	} else {
		if zitiAdminKey == nil {
			klog.Error(&MissingEnvVarError{variable: "ZITI_CTRL_ADMIN_KEY"})
			klog.Error(&MissingCmdLineVarError{variable: "ZITI_CTRL_ADMIN_KEY"})
		}
	}

	value, ok = os.LookupEnv("POD_SECURITY_CONTEXT_OVERRIDE")
	if ok && len(value) > 0 {
		podSecurityOverride, err = strconv.ParseBool(value)
		if err != nil {
			klog.Info(err)
		}
	}

	value, ok = os.LookupEnv("CLUSTER_DNS_SVC_IP")
	if ok && len(value) > 0 {
		clusterDnsServiceIP = value
	} else {
		if len(clusterDnsServiceIP) == 0 {
			klog.Error(&MissingEnvVarError{variable: "CLUSTER_DNS_SVC_IP"})
			klog.Error(&MissingCmdLineVarError{variable: "CLUSTER_DNS_SVC_IP"})
			klog.Infof(fmt.Sprintf("Custom DNS Server IP not set, Cluster DNS IP will be used instead"))
		}
	}

	value, ok = os.LookupEnv("SEARCH_DOMAIN_LIST")
	if ok && len(value) > 0 {
		searchDomains = []string(strings.Split(value, " "))
	} else {
		if len(searchDomainList) == 0 {
			klog.Error(&MissingEnvVarError{variable: "SEARCH_DOMAIN_LIST"})
			klog.Error(&MissingCmdLineVarError{variable: "SEARCH_DOMAIN_LIST"})
			klog.Infof(fmt.Sprintf("Custom DNS search domains not set, Kubernetes default domains will be used instead"))
		} else {
			searchDomains = []string(strings.Split(searchDomainList, " "))
		}
	}

	value, ok = os.LookupEnv("ZITI_ROLE_KEY")
	if ok && len(value) > 0 {
		zitiRoleKey = value
	} else {
		if len(zitiRoleKey) == 0 {
			klog.Error(&MissingEnvVarError{variable: "ZITI_ROLE_KEY"})
			klog.Error(&MissingCmdLineVarError{variable: "ZITI_ROLE_KEY"})
		}
	}
}
