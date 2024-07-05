package webhook

import (
	"fmt"

	"github.com/spf13/cobra"
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
	zitiCtrlMgmtApi        string
	zitiCtrlClientCertFile string
	zitiCtrlClientKeyFile  string
	podSecurityOverride    bool
	clusterDnsServiceIP    string
	searchDomainList       string
	searchDomains          []string
	zitiIdentityRoles      []string
	zitiRoleKey            string
	value                  string
	ok                     bool
	err                    error
)

func NewWebhookCmd() *cobra.Command {
	var webhookCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Starts a HTTP server,  Mutating Admission Webhook for injecting ziti sidecars",
		Long: `
Starts a HTTP server, Mutating Admission Webhook for injecting ziti sidecar proxy.
After deployed to kubernetes clusters, it listens to events related to pod CRUD operations 
and takes appropriate actions, i.e. create/delete ziti identity, secret, etc.`,
		Run: webhook,
	}

	webhookCmd.Flags().StringVar(&certFile, "tls-cert-file", "",
		"File containing the default x509 Certificate for HTTPS.")
	webhookCmd.Flags().StringVar(&keyFile, "tls-private-key-file", "",
		"File containing the default x509 private key matching --tls-cert-file.")
	webhookCmd.Flags().IntVar(&port, "port", 9443,
		"Secure port that the webhook listens on")
	webhookCmd.Flags().StringVar(&sidecarImage, "sidecar-image", "openziti/ziti-tunnel",
		"Image of sidecar")
	webhookCmd.Flags().StringVar(&sidecarImageVersion, "sidecar-image-version", "latest",
		"Image Varsion of sidecar")
	webhookCmd.Flags().StringVar(&sidecarPrefix, "sidecar-prefix", "zt",
		"Used in creation of ContainerName to be used as injected sidecar")
	webhookCmd.Flags().StringVar(&zitiCtrlMgmtApi, "ziti-ctrl-addr", "",
		"Ziti Controller Management URL, i.e. https://{FQDN}:{PORT}/edge/management/v1 ")
	webhookCmd.Flags().StringVar(&zitiCtrlClientCertFile, "ziti-ctrl-client-cert-file", "",
		"Ziti Controller Client Certificate")
	webhookCmd.Flags().StringVar(&zitiCtrlClientKeyFile, "ziti-ctrl-client-key-file", "",
		"Ziti Controller Client Private Key")
	webhookCmd.Flags().BoolVar(&podSecurityOverride, "pod-sc-override", false,
		"Override the security context at pod level, i.e. runAsNonRoot: false")
	webhookCmd.Flags().StringVar(&clusterDnsServiceIP, "cluster-dns-svc-ip", "",
		"Cluster DNS Service IP")
	webhookCmd.Flags().StringVar(&searchDomainList, "search-domain-list", "",
		"A list of DNS search domains as space seperated string i.e. 'value1 value2'")
	webhookCmd.Flags().StringVar(&zitiRoleKey, "ziti-role-key", "",
		"Ziti Identity Role Key used in pod annotation")

	return webhookCmd
}

func Execute() {
	if err := NewWebhookCmd().Execute(); err != nil {
		fmt.Printf("error: %s\n", err)
	}
}
