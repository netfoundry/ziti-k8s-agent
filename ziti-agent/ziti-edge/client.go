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
    klog.V(5).Infof("Creating Ziti Edge Management client with endpoint: %s, cert subject: %s, and CA pool with %d entries",
        cfg.ApiEndpoint, 
        cfg.Cert.Subject, 
        len(cfg.CAS.Subjects()))

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