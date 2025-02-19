package zitiedge

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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

    klog.V(4).Info("Client Certificate Details:")
    klog.V(4).Infof("  Subject: %v", cfg.Cert.Subject)
    klog.V(4).Infof("  Issuer: %v", cfg.Cert.Issuer)
    klog.V(4).Infof("  Valid from: %v to %v", cfg.Cert.NotBefore, cfg.Cert.NotAfter)
    klog.V(4).Infof("  Key usages: %s", keyUsageString(cfg.Cert.KeyUsage))
    klog.V(4).Infof("  Extended Key usages: %s", extKeyUsageString(cfg.Cert.ExtKeyUsage))
    klog.V(4).Info("CA Pool Certificate Details:")
    // Log the key usages of the CA certificate
    block, _ := pem.Decode(cfg.CABundle)
    if block == nil {
        klog.Error("Failed to decode PEM data")
    } else {
        parsedCert, err := x509.ParseCertificate(block.Bytes)
        if err != nil {
            klog.Errorf("Error parsing CA certificate: %v", err)
        } else {
            klog.V(4).Infof("  CA Subject: %v", parsedCert.Subject)
            klog.V(4).Infof("  CA Issuer: %v", parsedCert.Issuer)
            klog.V(4).Infof("  CA Valid from: %v to %v", parsedCert.NotBefore, parsedCert.NotAfter)
            klog.V(4).Infof("  CA Key usages: %s", keyUsageString(parsedCert.KeyUsage))
            klog.V(4).Infof("  CA Extended Key usages: %s", extKeyUsageString(parsedCert.ExtKeyUsage))
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
    ok, err := rest_util.VerifyController(cfg.ApiEndpoint, &cfg.CAS)
    if !ok {
        klog.Errorf("Ziti Controller failed CA validation - %s", err)
        return nil, errors.Wrap(err, "controller verification failed")
    }
    klog.V(5).Info("Controller verification successful")

    klog.V(5).Info("Creating new Edge Management client with certificate...")
    client, err := rest_util.NewEdgeManagementClientWithCert(cfg.Cert, cfg.PrivateKey, cfg.ApiEndpoint, &cfg.CAS)
    if err != nil {
        return nil, errors.Wrap(err, "failed to create edge management client")
    }
    klog.V(5).Info("Successfully created Edge Management client")

    return client, nil
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