package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
)

// Signer signs messages to prove authenticity of host agent communications.
type Signer struct {
	privateKey *rsa.PrivateKey
}

// NewSigner loads the host's private key for message signing.
func NewSigner(keyPath string) (*Signer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, &SecurityError{Message: "failed to parse PEM block"}
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, &SecurityError{Message: "private key is not RSA"}
	}

	return &Signer{privateKey: rsaKey}, nil
}

// Sign creates a digital signature for the given data.
func (s *Signer) Sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash[:])
}
