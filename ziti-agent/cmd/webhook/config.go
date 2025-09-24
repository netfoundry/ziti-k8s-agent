package webhook

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type WebhookConfig struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Controller struct {
		MgmtAPI string `yaml:"mgmtApi"`
		RoleKey string `yaml:"roleKey"`
	} `yaml:"controller"`

	Sidecar struct {
		Image              string   `yaml:"image"`
		ImageVersion       string   `yaml:"imageVersion"`
		ImagePullPolicy    string   `yaml:"imagePullPolicy"`
		Prefix             string   `yaml:"prefix"`
		IdentityDir        string   `yaml:"identityDir"`
		VolumeMountName    string   `yaml:"volumeMountName"`
		ResolverIP         string   `yaml:"resolverIp"`
		DnsUpstreamEnabled bool     `yaml:"dnsUpstreamEnabled"`
		DnsUnanswerable    string   `yaml:"dnsUnanswerable"`
		SearchDomains      []string `yaml:"searchDomains"`
	} `yaml:"sidecar"`

	Security struct {
		PodSecurityContextOverride bool `yaml:"podSecurityContextOverride"`
	} `yaml:"security"`
}

func loadConfig(path string) (*WebhookConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg WebhookConfig
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyConfigDefaults(&cfg)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyConfigDefaults(cfg *WebhookConfig) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9443
	}

	if cfg.Sidecar.ImagePullPolicy == "" {
		cfg.Sidecar.ImagePullPolicy = defaultImagePullPolicy
	}

	if cfg.Sidecar.VolumeMountName == "" {
		cfg.Sidecar.VolumeMountName = "ziti-identity"
	}

	if cfg.Sidecar.IdentityDir == "" {
		cfg.Sidecar.IdentityDir = "/ziti-tunnel"
	}

	if cfg.Sidecar.Prefix == "" {
		cfg.Sidecar.Prefix = "zt"
	}

	if cfg.Sidecar.DnsUnanswerable == "" {
		cfg.Sidecar.DnsUnanswerable = "refused"
	}

	if cfg.Controller.RoleKey == "" {
		cfg.Controller.RoleKey = defaultZitiRoleAttributesKey
	}
}

func validateConfig(cfg *WebhookConfig) error {
	if cfg.Controller.MgmtAPI == "" {
		return errors.New("controller.mgmtApi is required")
	}

	if cfg.Sidecar.Image == "" {
		return errors.New("sidecar.image is required")
	}

	if cfg.Sidecar.ImageVersion == "" {
		return errors.New("sidecar.imageVersion is required")
	}

	return nil
}

func configTLS(cert, key []byte) *tls.Config {
	sCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		klog.Fatalf("failed to load webhook server TLS key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{sCert}}
}

func loadCertificatesFromEnv() {
	if value, ok := os.LookupEnv("TLS_CERT"); ok && value != "" {
		cert = []byte(value)
	}

	if value, ok := os.LookupEnv("TLS_PRIVATE_KEY"); ok && value != "" {
		key = []byte(value)
	}

	if value, ok := os.LookupEnv("ZITI_ADMIN_CERT"); ok && value != "" {
		zitiAdminCert = []byte(value)
	}

	if value, ok := os.LookupEnv("ZITI_ADMIN_KEY"); ok && value != "" {
		zitiAdminKey = []byte(value)
	}

	if value, ok := os.LookupEnv("ZITI_CTRL_CA_BUNDLE"); ok && value != "" {
		zitiCtrlCaBundle = []byte(value)
	}
}
