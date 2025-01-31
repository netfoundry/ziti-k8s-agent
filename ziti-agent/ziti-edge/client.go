package zitiedge

import (
	"crypto"
	"crypto/x509"

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

    klog.V(4).Info("CA Pool Certificate Details:")
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