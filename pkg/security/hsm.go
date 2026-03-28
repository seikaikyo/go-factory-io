package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"sync"
)

// KeyStore is the interface for hardware security module (HSM) key storage.
// TLS private keys are stored in the HSM and never leave the device.
// Implementations may use PKCS#11, TPM, AWS CloudHSM, etc.
type KeyStore interface {
	// Sign signs a digest using the private key identified by keyID.
	Sign(keyID string, digest []byte, opts crypto.SignerOpts) ([]byte, error)

	// GetCertificate returns the certificate associated with a key.
	GetCertificate(keyID string) (*x509.Certificate, error)

	// GetSigner returns a crypto.Signer backed by the HSM for a given key.
	// Used with tls.Config.Certificates for TLS handshake.
	GetSigner(keyID string) (crypto.Signer, error)

	// ListKeys returns all available key identifiers.
	ListKeys() []string
}

// SoftwareKeyStore is an in-memory KeyStore implementation for testing.
// Not suitable for production -- keys are stored in process memory.
type SoftwareKeyStore struct {
	mu    sync.RWMutex
	keys  map[string]*softKey
}

type softKey struct {
	signer crypto.Signer
	cert   *x509.Certificate
}

// NewSoftwareKeyStore creates an in-memory key store.
func NewSoftwareKeyStore() *SoftwareKeyStore {
	return &SoftwareKeyStore{
		keys: make(map[string]*softKey),
	}
}

// GenerateKey generates a new ECDSA P-256 key pair and stores it.
func (ks *SoftwareKeyStore) GenerateKey(keyID string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("hsm: generate key: %w", err)
	}

	ks.mu.Lock()
	ks.keys[keyID] = &softKey{signer: key}
	ks.mu.Unlock()
	return nil
}

// ImportKey imports an existing crypto.Signer.
func (ks *SoftwareKeyStore) ImportKey(keyID string, signer crypto.Signer, cert *x509.Certificate) {
	ks.mu.Lock()
	ks.keys[keyID] = &softKey{signer: signer, cert: cert}
	ks.mu.Unlock()
}

// Sign implements KeyStore.Sign.
func (ks *SoftwareKeyStore) Sign(keyID string, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ks.mu.RLock()
	k, ok := ks.keys[keyID]
	ks.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("hsm: key %s not found", keyID)
	}
	return k.signer.Sign(rand.Reader, digest, opts)
}

// GetCertificate implements KeyStore.GetCertificate.
func (ks *SoftwareKeyStore) GetCertificate(keyID string) (*x509.Certificate, error) {
	ks.mu.RLock()
	k, ok := ks.keys[keyID]
	ks.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("hsm: key %s not found", keyID)
	}
	if k.cert == nil {
		return nil, fmt.Errorf("hsm: key %s has no certificate", keyID)
	}
	return k.cert, nil
}

// GetSigner implements KeyStore.GetSigner.
func (ks *SoftwareKeyStore) GetSigner(keyID string) (crypto.Signer, error) {
	ks.mu.RLock()
	k, ok := ks.keys[keyID]
	ks.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("hsm: key %s not found", keyID)
	}
	return k.signer, nil
}

// ListKeys implements KeyStore.ListKeys.
func (ks *SoftwareKeyStore) ListKeys() []string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	keys := make([]string, 0, len(ks.keys))
	for id := range ks.keys {
		keys = append(keys, id)
	}
	return keys
}

// TLSCertificate creates a tls.Certificate backed by the key store.
// The private key operations are delegated to the KeyStore (HSM-compatible).
func (ks *SoftwareKeyStore) TLSCertificate(keyID string) (*tls.Certificate, error) {
	ks.mu.RLock()
	k, ok := ks.keys[keyID]
	ks.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("hsm: key %s not found", keyID)
	}
	if k.cert == nil {
		return nil, fmt.Errorf("hsm: key %s has no certificate", keyID)
	}

	return &tls.Certificate{
		Certificate: [][]byte{k.cert.Raw},
		PrivateKey:  k.signer,
		Leaf:        k.cert,
	}, nil
}

// hsmSigner wraps a KeyStore key as a crypto.Signer.
// Used when the KeyStore doesn't directly expose the key.
type hsmSigner struct {
	keyStore KeyStore
	keyID    string
	pub      crypto.PublicKey
}

// NewHSMSigner creates a crypto.Signer backed by an HSM KeyStore.
func NewHSMSigner(ks KeyStore, keyID string, pub crypto.PublicKey) crypto.Signer {
	return &hsmSigner{keyStore: ks, keyID: keyID, pub: pub}
}

func (s *hsmSigner) Public() crypto.PublicKey {
	return s.pub
}

func (s *hsmSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return s.keyStore.Sign(s.keyID, digest, opts)
}
