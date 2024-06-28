package zitiEdge

import (
	"crypto"
	"crypto/x509"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_util"
	"k8s.io/klog/v2"
)

type Config struct {
	ApiEndpoint string
	Cert        *x509.Certificate
	PrivateKey  crypto.PrivateKey
	CAS         x509.CertPool
}

func Client(cfg *Config) (*rest_management_api_client.ZitiEdgeManagement, error) {
	caCerts, err := rest_util.GetControllerWellKnownCas(cfg.ApiEndpoint)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	for _, ca := range caCerts {
		caPool.AddCert(ca)
	}

	ok, err := rest_util.VerifyController(cfg.ApiEndpoint, caPool)
	if !ok {
		klog.Errorf("Ziti Controller failed CA validation - %s", err)
		return nil, err
	}

	//return rest_util.NewEdgeManagementClientWithUpdb(cfg.Username, cfg.Password, cfg.ApiEndpoint, caPool)
	return rest_util.NewEdgeManagementClientWithCert(cfg.Cert, cfg.PrivateKey, cfg.ApiEndpoint, caPool)
}
