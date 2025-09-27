package webhook

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// ZitiIdentityConfig represents the standard Ziti identity configuration JSON structure
type ZitiIdentityConfig struct {
	ID struct {
		CA   string `json:"ca"`
		Cert string `json:"cert"`
		Key  string `json:"key"`
	} `json:"id"`
	ZtAPI  string   `json:"ztAPI"`
	ZtAPIs []string `json:"ztAPIs"`
}

type WebhookConfig struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Controller struct {
		MgmtAPI string `yaml:"mgmtApi"` // Optional - if empty, will be inferred from identity
		RoleKey string `yaml:"roleKey"`
		// Runtime fields populated during config loading
		MgmtAPIEndpoints []string `yaml:"-"` // List of management API endpoints to try
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

	ClusterDns struct {
		Zone string `yaml:"zone"`
	} `yaml:"clusterDns"`

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
	
	// If mgmtApi is not specified, infer from identity configuration
	if cfg.Controller.MgmtAPI == "" {
		if err := inferMgmtAPIEndpoints(&cfg); err != nil {
			return nil, fmt.Errorf("failed to infer management API endpoints: %w", err)
		}
	} else {
		// Use the explicitly configured endpoint
		cfg.Controller.MgmtAPIEndpoints = []string{cfg.Controller.MgmtAPI}
	}
	
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

	if cfg.ClusterDns.Zone == "" {
		cfg.ClusterDns.Zone = "cluster.local"
	}

}

func validateConfig(cfg *WebhookConfig) error {
	// Check that we have at least one source of management API endpoints
	// Priority: 1) explicit mgmtApi config, 2) inferred from identity (ztAPI or ztAPIs)
	if len(cfg.Controller.MgmtAPIEndpoints) == 0 {
		return errors.New("no management API endpoints available - must specify one of: 1) controller.mgmtApi in webhook config, 2) ztAPI in identity JSON, or 3) ztAPIs (non-empty) in identity JSON")
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

// loadZitiIdentityFromEnv loads the Ziti identity configuration from ZITI_IDENTITY_JSON environment variable
func loadZitiIdentityFromEnv() (*ZitiIdentityConfig, error) {
	identityJSON, ok := os.LookupEnv("ZITI_IDENTITY_JSON")
	if !ok || identityJSON == "" {
		return nil, errors.New("ZITI_IDENTITY_JSON environment variable is required and must contain valid JSON")
	}

	var identity ZitiIdentityConfig
	if err := json.Unmarshal([]byte(identityJSON), &identity); err != nil {
		return nil, fmt.Errorf("failed to parse Ziti identity JSON from environment variable: %w", err)
	}

	// Validate required fields
	if identity.ID.CA == "" {
		return nil, errors.New("ziti identity missing required field: id.ca")
	}
	if identity.ID.Cert == "" {
		return nil, errors.New("ziti identity missing required field: id.cert")
	}
	if identity.ID.Key == "" {
		return nil, errors.New("ziti identity missing required field: id.key")
	}

	klog.V(4).Infof("Successfully loaded Ziti identity from ZITI_IDENTITY_JSON environment variable")
	return &identity, nil
}

// inferMgmtAPIEndpoints infers management API endpoints from the Ziti identity configuration
func inferMgmtAPIEndpoints(cfg *WebhookConfig) error {
	identity, err := loadZitiIdentityFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load Ziti identity for endpoint inference: %w", err)
	}

	var endpoints []string
	
	// Add primary ztAPI endpoint if available (ztAPI is optional)
	if identity.ZtAPI != "" {
		mgmtURL, err := convertToMgmtAPI(identity.ZtAPI)
		if err != nil {
			klog.V(2).Infof("Failed to convert ztAPI to management API URL: %v", err)
		} else {
			endpoints = append(endpoints, mgmtURL)
		}
	}
	
	// Add additional ztAPIs endpoints if available (ztAPIs is optional)
	if identity.ZtAPIs != nil {
		for _, apiURL := range identity.ZtAPIs {
			if apiURL != "" && apiURL != identity.ZtAPI { // Avoid duplicates
				mgmtURL, err := convertToMgmtAPI(apiURL)
				if err != nil {
					klog.V(2).Infof("Failed to convert ztAPI to management API URL: %v", err)
					continue
				}
				endpoints = append(endpoints, mgmtURL)
			}
		}
	}
	
	if len(endpoints) == 0 {
		return errors.New("no valid management API endpoints could be inferred from identity configuration - must specify one of: 1) controller.mgmtApi in webhook config, 2) ztAPI in identity JSON, or 3) ztAPIs (non-empty) in identity JSON")
	}
	
	cfg.Controller.MgmtAPIEndpoints = endpoints
	klog.V(2).Infof("Inferred %d management API endpoints from identity configuration: %v", len(endpoints), endpoints)
	
	return nil
}

// convertToMgmtAPI converts a Ziti client API URL to a management API URL
// Example: https://controller:1280/edge/client/v1 -> https://controller:1280/edge/management/v1
func convertToMgmtAPI(clientAPIURL string) (string, error) {
	parsedURL, err := url.Parse(clientAPIURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	
	// Replace /edge/client/v1 with /edge/management/v1
	path := parsedURL.Path
	if strings.Contains(path, "/edge/management/v1") {
		// Already a management API URL, use as-is
		// No changes needed
	} else if strings.HasSuffix(path, "/edge/client/v1") {
		path = strings.TrimSuffix(path, "/edge/client/v1") + "/edge/management/v1"
	} else if strings.Contains(path, "/edge/client/v1") {
		// Handle other client API versions
		path = strings.Replace(path, "/edge/client/v1", "/edge/management/v1", 1)
	} else {
		// If it doesn't look like a client API URL, assume it's a base URL and append management path
		path = strings.TrimSuffix(path, "/") + "/edge/management/v1"
	}
	
	parsedURL.Path = path
	return parsedURL.String(), nil
}

// loadWebhookTLSFromEnv loads webhook server TLS certificates from environment variables
func loadWebhookTLSFromEnv() {
	if value, ok := os.LookupEnv("TLS_CERT"); ok && value != "" {
		cert = []byte(value)
	}

	if value, ok := os.LookupEnv("TLS_PRIVATE_KEY"); ok && value != "" {
		key = []byte(value)
	}
}
