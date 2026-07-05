package security

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

// LoadTLSConfig creates a mutual TLS configuration for secure communication
// with the gateway/scheduler.
// SECURITY: mTLS ensures both parties are authenticated:
// - Host proves identity to gateway (client cert)
// - Host verifies gateway is genuine (CA verification)
func LoadTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	// Load host's client certificate
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	// Load CA certificate to verify the gateway
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, &SecurityError{Message: "failed to parse CA certificate"}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13, // SECURITY: Require TLS 1.3 minimum
	}, nil
}

type SecurityError struct {
	Message string
}

func (e *SecurityError) Error() string {
	return e.Message
}
