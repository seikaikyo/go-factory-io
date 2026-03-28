package security

import (
	"crypto/x509"
	"math/big"
	"testing"
	"time"
)

func TestRevocationCheckerNoCRL(t *testing.T) {
	rc := NewRevocationChecker(nil, time.Hour)

	// Certificate with no CRL distribution points
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}

	err := rc.CheckCRL(cert)
	if err != nil {
		t.Errorf("expected nil for cert with no CRL endpoints, got: %v", err)
	}
}

func TestRevocationCheckerNoOCSP(t *testing.T) {
	rc := NewRevocationChecker(nil, time.Hour)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	issuer := &x509.Certificate{}

	err := rc.CheckOCSP(cert, issuer)
	if err != nil {
		t.Errorf("expected nil for cert with no OCSP servers, got: %v", err)
	}
}

func TestRevocationCheckerVerifyNoCRL(t *testing.T) {
	rc := NewRevocationChecker(nil, time.Hour)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(42),
	}

	// Should pass when no CRL/OCSP endpoints are configured
	err := rc.VerifyCertificate(cert, nil)
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestRevocationCheckerClearCache(t *testing.T) {
	rc := NewRevocationChecker(nil, time.Hour)
	rc.crlCache["http://example.com/crl"] = &crlEntry{
		list:      &x509.RevocationList{},
		fetchedAt: time.Now(),
	}

	rc.ClearCache()

	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if len(rc.crlCache) != 0 {
		t.Errorf("cache should be empty after clear, got %d entries", len(rc.crlCache))
	}
}

func TestRevocationCheckerDefaults(t *testing.T) {
	rc := NewRevocationChecker(nil, 0)
	if rc.cacheTTL != time.Hour {
		t.Errorf("default cacheTTL = %v, want 1h", rc.cacheTTL)
	}
}
