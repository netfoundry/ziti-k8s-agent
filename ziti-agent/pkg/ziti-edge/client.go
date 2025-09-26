package zitiedge

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_util"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type Config struct {
	ApiEndpoint string
	Cert        *x509.Certificate
	PrivateKey  crypto.PrivateKey
	CAS         x509.CertPool
	CABundle    []byte
}

// Create a Ziti Edge API session with a Ziti Identity configuration
func Client(cfg *Config) (*rest_management_api_client.ZitiEdgeManagement, error) {
	klog.V(5).Infof("Creating Ziti Edge Management client with endpoint: %s, cert subject: %s",
		cfg.ApiEndpoint,
		cfg.Cert.Subject,
	)

	klog.V(5).Info("Client Certificate Details:")
	klog.V(5).Infof("  Subject: %v", cfg.Cert.Subject)
	klog.V(5).Infof("  Issuer: %v", cfg.Cert.Issuer)
	klog.V(5).Infof("  Valid from: %v to %v", cfg.Cert.NotBefore, cfg.Cert.NotAfter)
	klog.V(5).Infof("  Key usages: %s", keyUsageString(cfg.Cert.KeyUsage))
	klog.V(5).Infof("  Extended Key usages: %s", extKeyUsageString(cfg.Cert.ExtKeyUsage))
	klog.V(5).Info("CA Pool Certificate Details:")
	// Log the key usages of the CA certificate
	block, _ := pem.Decode(cfg.CABundle)
	if block == nil {
		klog.Error("Failed to decode PEM data")
	} else {
		parsedCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			klog.Errorf("Error parsing CA certificate: %v", err)
		} else {
			klog.V(5).Infof("  CA Subject: %v", parsedCert.Subject)
			klog.V(5).Infof("  CA Issuer: %v", parsedCert.Issuer)
			klog.V(5).Infof("  CA Valid from: %v to %v", parsedCert.NotBefore, parsedCert.NotAfter)
			klog.V(5).Infof("  CA Key usages: %s", keyUsageString(parsedCert.KeyUsage))
			klog.V(5).Infof("  CA Extended Key usages: %s", extKeyUsageString(parsedCert.ExtKeyUsage))
		}
	}
	// Check if our client cert is trusted by the CA pool
	opts := x509.VerifyOptions{
		Roots: &cfg.CAS,
	}
	if _, err := cfg.Cert.Verify(opts); err == nil {
		klog.V(4).Info("Client certificate is trusted by the CA pool")
	} else {
		klog.V(4).Infof("Warning: Client certificate is not trusted by the CA pool: %v", err)
	}

	klog.V(5).Info("Verifying controller with provided CA pool...")
	
	// Extract base controller URL for certificate verification
	// VerifyController appends /edge/client/v1/versions, so we need the base URL
	baseControllerURL, err := extractControllerBaseURL(cfg.ApiEndpoint)
	if err != nil {
		klog.Errorf("Failed to extract base controller URL: %v", err)
		return nil, errors.Wrap(err, "failed to extract base controller URL")
	}
	
	klog.V(5).Infof("Using base controller URL for verification: %s", baseControllerURL)
	ok, err := rest_util.VerifyController(baseControllerURL, &cfg.CAS)
	if !ok {
		klog.Errorf("Ziti Controller failed CA validation - %s", err)
		return nil, errors.Wrap(err, "controller verification failed")
	}
	klog.V(5).Info("Controller verification successful")

	// Reconstitute the management API URL from the sanitized base URL
	mgmtAPIURL := reconstituteMgmtAPIURL(baseControllerURL)
	klog.V(5).Infof("Using reconstituted management API URL: %s", mgmtAPIURL)

	klog.V(5).Info("Creating new Edge Management client with certificate...")
	client, err := rest_util.NewEdgeManagementClientWithCert(cfg.Cert, cfg.PrivateKey, mgmtAPIURL, &cfg.CAS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create edge management client")
	}
	klog.V(5).Info("Successfully created Edge Management client")

	return client, nil
}

// extractControllerBaseURL extracts the base controller URL for certificate verification
// by removing ALL URL path components and conditionally removing -p suffix from NetFoundry hostnames
// VerifyController expects just the base URL since it appends /edge/client/v1/versions
func extractControllerBaseURL(mgmtAPIURL string) (string, error) {
	parsedURL, err := url.Parse(mgmtAPIURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse management API URL: %w", err)
	}

	hostname := parsedURL.Hostname()
	port := parsedURL.Port()

	// Check if hostname matches NetFoundry pattern: <uuid>-p.<env>.netfoundry.io
	// If so, remove the -p suffix for certificate verification
	if strings.Contains(hostname, ".netfoundry.io") && strings.Contains(hostname, "-p.") {
		hostname = strings.Replace(hostname, "-p.", ".", 1)
	}

	// Construct base URL with scheme, cleaned hostname, and port, but NO path
	// VerifyController will append /edge/client/v1/versions to this base URL
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, hostname)
	if port != "" {
		baseURL += ":" + port
	}

	return baseURL, nil
}

// reconstituteMgmtAPIURL takes a sanitized base controller URL and reconstitutes the management API endpoint
// Only appends /edge/management/v1 if it's not already present
func reconstituteMgmtAPIURL(baseControllerURL string) string {
	if strings.HasSuffix(baseControllerURL, "/edge/management/v1") {
		return baseControllerURL
	}
	return baseControllerURL + "/edge/management/v1"
}

func keyUsageString(ku x509.KeyUsage) string {
	var usages []string
	if ku&x509.KeyUsageDigitalSignature != 0 {
		usages = append(usages, "DigitalSignature")
	}
	if ku&x509.KeyUsageContentCommitment != 0 {
		usages = append(usages, "ContentCommitment")
	}
	if ku&x509.KeyUsageKeyEncipherment != 0 {
		usages = append(usages, "KeyEncipherment")
	}
	if ku&x509.KeyUsageDataEncipherment != 0 {
		usages = append(usages, "DataEncipherment")
	}
	if ku&x509.KeyUsageKeyAgreement != 0 {
		usages = append(usages, "KeyAgreement")
	}
	if ku&x509.KeyUsageCertSign != 0 {
		usages = append(usages, "CertSign")
	}
	if ku&x509.KeyUsageCRLSign != 0 {
		usages = append(usages, "CRLSign")
	}
	if ku&x509.KeyUsageEncipherOnly != 0 {
		usages = append(usages, "EncipherOnly")
	}
	if ku&x509.KeyUsageDecipherOnly != 0 {
		usages = append(usages, "DecipherOnly")
	}
	return fmt.Sprintf("[%s]", strings.Join(usages, ", "))
}

func extKeyUsageString(eku []x509.ExtKeyUsage) string {
	var usages []string
	for _, ku := range eku {
		switch ku {
		case x509.ExtKeyUsageAny:
			usages = append(usages, "Any")
		case x509.ExtKeyUsageServerAuth:
			usages = append(usages, "ServerAuth")
		case x509.ExtKeyUsageClientAuth:
			usages = append(usages, "ClientAuth")
		case x509.ExtKeyUsageCodeSigning:
			usages = append(usages, "CodeSigning")
		case x509.ExtKeyUsageEmailProtection:
			usages = append(usages, "EmailProtection")
		case x509.ExtKeyUsageIPSECEndSystem:
			usages = append(usages, "IPSECEndSystem")
		case x509.ExtKeyUsageIPSECTunnel:
			usages = append(usages, "IPSECTunnel")
		case x509.ExtKeyUsageIPSECUser:
			usages = append(usages, "IPSECUser")
		case x509.ExtKeyUsageTimeStamping:
			usages = append(usages, "TimeStamping")
		case x509.ExtKeyUsageOCSPSigning:
			usages = append(usages, "OCSPSigning")
		case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
			usages = append(usages, "MicrosoftServerGatedCrypto")
		case x509.ExtKeyUsageNetscapeServerGatedCrypto:
			usages = append(usages, "NetscapeServerGatedCrypto")
		case x509.ExtKeyUsageMicrosoftCommercialCodeSigning:
			usages = append(usages, "MicrosoftCommercialCodeSigning")
		default:
			usages = append(usages, fmt.Sprintf("Unknown (%d)", ku))
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(usages, ", "))
}
