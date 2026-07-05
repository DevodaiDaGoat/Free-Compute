package security

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
)

// Verifier verifies signatures from the gateway/scheduler.
type Verifier struct {
	publicKey *rsa.PublicKey
}

// NewVerifier loads the gateway's public key for signature verification.
func NewVerifier(pubKeyPath string) (*Verifier, error) {
	keyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, &SecurityError{Message: "failed to parse PEM block"}
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, &SecurityError{Message: "public key is not RSA"}
	}

	return &Verifier{publicKey: rsaKey}, nil
}

// Verify checks that data was signed by the expected party (gateway).
func (v *Verifier) Verify(data, signature []byte) error {
	hash := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(v.publicKey, crypto.SHA256, hash[:], signature)
}
