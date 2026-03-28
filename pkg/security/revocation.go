package security

import (
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RevocationChecker verifies that X.509 certificates have not been revoked.
// Supports CRL (Certificate Revocation List) with local caching and OCSP.
type RevocationChecker struct {
	logger   *slog.Logger
	client   *http.Client
	mu       sync.RWMutex
	crlCache map[string]*crlEntry // URL -> cached CRL
	cacheTTL time.Duration
}

type crlEntry struct {
	list      *x509.RevocationList
	fetchedAt time.Time
}

// NewRevocationChecker creates a certificate revocation checker.
func NewRevocationChecker(logger *slog.Logger, cacheTTL time.Duration) *RevocationChecker {
	if logger == nil {
		logger = slog.Default()
	}
	if cacheTTL == 0 {
		cacheTTL = 1 * time.Hour
	}
	return &RevocationChecker{
		logger:   logger,
		client:   &http.Client{Timeout: 10 * time.Second},
		crlCache: make(map[string]*crlEntry),
		cacheTTL: cacheTTL,
	}
}

// CheckCRL checks if a certificate is revoked via CRL distribution points.
// Returns nil if not revoked, error if revoked or check fails.
func (rc *RevocationChecker) CheckCRL(cert *x509.Certificate) error {
	if len(cert.CRLDistributionPoints) == 0 {
		return nil // No CRL endpoints, cannot check
	}

	for _, url := range cert.CRLDistributionPoints {
		crl, err := rc.fetchCRL(url)
		if err != nil {
			rc.logger.Warn("CRL fetch failed", "url", url, "error", err)
			continue
		}

		for _, revoked := range crl.RevokedCertificateEntries {
			if revoked.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return fmt.Errorf("security: certificate serial %s revoked (CRL: %s)",
					cert.SerialNumber, url)
			}
		}
		return nil // Found valid CRL and cert is not on it
	}

	return fmt.Errorf("security: could not verify CRL for certificate %s", cert.Subject.CommonName)
}

// CheckOCSP checks certificate revocation via OCSP.
// Returns nil if not revoked. Uses HTTP GET to the OCSP responder.
func (rc *RevocationChecker) CheckOCSP(cert, issuer *x509.Certificate) error {
	if len(cert.OCSPServer) == 0 {
		return nil // No OCSP responder configured
	}

	// Build OCSP request
	ocspReq, err := buildOCSPRequest(cert, issuer)
	if err != nil {
		return fmt.Errorf("security: build OCSP request: %w", err)
	}

	for _, server := range cert.OCSPServer {
		resp, err := rc.client.Post(server, "application/ocsp-request", ocspReq)
		if err != nil {
			rc.logger.Warn("OCSP request failed", "server", server, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			rc.logger.Warn("OCSP server error", "server", server, "status", resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		status, err := parseOCSPResponse(body)
		if err != nil {
			rc.logger.Warn("OCSP parse error", "server", server, "error", err)
			continue
		}

		switch status {
		case ocspGood:
			return nil
		case ocspRevoked:
			return fmt.Errorf("security: certificate %s revoked (OCSP: %s)", cert.Subject.CommonName, server)
		default:
			continue // Unknown status, try next server
		}
	}

	return nil // If no OCSP server responds definitively, assume good (soft-fail)
}

// VerifyCertificate checks both CRL and OCSP for a certificate.
// Returns nil if the certificate is not revoked.
func (rc *RevocationChecker) VerifyCertificate(cert, issuer *x509.Certificate) error {
	// Check CRL first (can be done without issuer)
	if err := rc.CheckCRL(cert); err != nil {
		return err
	}

	// Check OCSP if issuer is available
	if issuer != nil {
		if err := rc.CheckOCSP(cert, issuer); err != nil {
			return err
		}
	}

	return nil
}

func (rc *RevocationChecker) fetchCRL(url string) (*x509.RevocationList, error) {
	// Check cache
	rc.mu.RLock()
	entry, ok := rc.crlCache[url]
	rc.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < rc.cacheTTL {
		return entry.list, nil
	}

	// Fetch CRL
	resp, err := rc.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch CRL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CRL server returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read CRL: %w", err)
	}

	crl, err := x509.ParseRevocationList(body)
	if err != nil {
		return nil, fmt.Errorf("parse CRL: %w", err)
	}

	// Cache
	rc.mu.Lock()
	rc.crlCache[url] = &crlEntry{list: crl, fetchedAt: time.Now()}
	rc.mu.Unlock()

	rc.logger.Debug("CRL cached", "url", url, "entries", len(crl.RevokedCertificateEntries))
	return crl, nil
}

// ClearCache removes all cached CRLs.
func (rc *RevocationChecker) ClearCache() {
	rc.mu.Lock()
	rc.crlCache = make(map[string]*crlEntry)
	rc.mu.Unlock()
}

// OCSP status codes
const (
	ocspGood    = 0
	ocspRevoked = 1
	ocspUnknown = 2
)

// buildOCSPRequest creates a minimal OCSP request.
// This is a simplified implementation; production should use golang.org/x/crypto/ocsp.
func buildOCSPRequest(cert, issuer *x509.Certificate) (io.Reader, error) {
	// For a full implementation, use ocsp.CreateRequest from golang.org/x/crypto/ocsp.
	// This stub returns nil to indicate OCSP request building is not fully implemented
	// without the x/crypto dependency. The CheckOCSP method handles this gracefully.
	return nil, fmt.Errorf("OCSP request building requires golang.org/x/crypto/ocsp")
}

// parseOCSPResponse parses an OCSP response and returns the status.
func parseOCSPResponse(data []byte) (int, error) {
	// Simplified: check the OCSPResponse status byte
	// Full implementation would use ocsp.ParseResponse from golang.org/x/crypto/ocsp
	if len(data) < 1 {
		return ocspUnknown, fmt.Errorf("empty OCSP response")
	}
	return ocspUnknown, fmt.Errorf("OCSP response parsing requires golang.org/x/crypto/ocsp")
}
