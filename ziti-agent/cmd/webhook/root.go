package webhook

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	// Default annotation and label keys
	defaultZitiRoleAttributesKey = "identity.openziti.io/role-attributes"
	defaultZitiTunnelLabelKey    = "tunnel.openziti.io/enabled"
	// Default values
	defaultImagePullPolicy = "IfNotPresent"
)

var (
	configPath       string
	cert             []byte
	key              []byte
	zitiAdminCert    []byte
	zitiAdminKey     []byte
	zitiCtrlCaBundle []byte
	runtimeConfig    *WebhookConfig
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

	webhookCmd.Flags().StringVar(&configPath, "config", "",
		"Path to the webhook configuration file")
	_ = webhookCmd.MarkFlagRequired("config")

	return webhookCmd
}

func Execute() {
	if err := NewWebhookCmd().Execute(); err != nil {
		fmt.Printf("error: %s\n", err)
	}
}
