package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestSoftwareKeyStoreGenerate(t *testing.T) {
	ks := NewSoftwareKeyStore()

	if err := ks.GenerateKey("test-key"); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	keys := ks.ListKeys()
	if len(keys) != 1 || keys[0] != "test-key" {
		t.Errorf("ListKeys = %v, want [test-key]", keys)
	}

	signer, err := ks.GetSigner("test-key")
	if err != nil {
		t.Fatalf("GetSigner: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestSoftwareKeyStoreSign(t *testing.T) {
	ks := NewSoftwareKeyStore()
	ks.GenerateKey("sign-key")

	digest := sha256.Sum256([]byte("test data"))
	sig, err := ks.Sign("sign-key", digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Error("signature is empty")
	}

	// Verify with public key
	signer, _ := ks.GetSigner("sign-key")
	pub := signer.Public().(*ecdsa.PublicKey)
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		t.Error("signature verification failed")
	}
}

func TestSoftwareKeyStoreImport(t *testing.T) {
	ks := NewSoftwareKeyStore()

	// Generate a key + self-signed cert
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(certDER)

	ks.ImportKey("imported", key, cert)

	gotCert, err := ks.GetCertificate("imported")
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if gotCert.Subject.CommonName != "test" {
		t.Errorf("CN = %s, want test", gotCert.Subject.CommonName)
	}
}

func TestSoftwareKeyStoreTLS(t *testing.T) {
	ks := NewSoftwareKeyStore()

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tls-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(certDER)

	ks.ImportKey("tls-key", key, cert)

	tlsCert, err := ks.TLSCertificate("tls-key")
	if err != nil {
		t.Fatalf("TLSCertificate: %v", err)
	}
	if tlsCert.Leaf.Subject.CommonName != "tls-test" {
		t.Errorf("leaf CN = %s, want tls-test", tlsCert.Leaf.Subject.CommonName)
	}
}

func TestSoftwareKeyStoreNotFound(t *testing.T) {
	ks := NewSoftwareKeyStore()

	if _, err := ks.Sign("nope", nil, nil); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := ks.GetCertificate("nope"); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := ks.GetSigner("nope"); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := ks.TLSCertificate("nope"); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestSoftwareKeyStoreNoCert(t *testing.T) {
	ks := NewSoftwareKeyStore()
	ks.GenerateKey("no-cert")

	if _, err := ks.GetCertificate("no-cert"); err == nil {
		t.Error("expected error for key without certificate")
	}
	if _, err := ks.TLSCertificate("no-cert"); err == nil {
		t.Error("expected error for key without certificate")
	}
}

func TestHSMSigner(t *testing.T) {
	ks := NewSoftwareKeyStore()
	ks.GenerateKey("hsm-key")
	signer, _ := ks.GetSigner("hsm-key")
	pub := signer.Public()

	hsmSigner := NewHSMSigner(ks, "hsm-key", pub)

	if hsmSigner.Public() != pub {
		t.Error("Public() should return the same key")
	}

	digest := sha256.Sum256([]byte("test"))
	sig, err := hsmSigner.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("HSMSigner.Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Error("signature is empty")
	}
}

func TestKeyStoreInterface(t *testing.T) {
	// Verify SoftwareKeyStore implements KeyStore
	var _ KeyStore = (*SoftwareKeyStore)(nil)
}
