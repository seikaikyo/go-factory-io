package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// DefaultTLSConfig returns a TLS config with secure defaults per IEC 62443 SR 4.3.
// - TLS 1.2 minimum
// - Strong cipher suites only (AES-GCM, ChaCha20)
// - Certificate verification enabled
func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}
}

// LoadServerTLS creates a TLS config for the server (passive/equipment) side.
// certFile and keyFile are the server's certificate and private key.
// caFile is optional; if provided, enables mutual TLS (client certificate verification).
func LoadServerTLS(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("security: load server cert: %w", err)
	}

	cfg := DefaultTLSConfig()
	cfg.Certificates = []tls.Certificate{cert}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("security: read CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("security: failed to parse CA cert")
		}
		cfg.ClientCAs = caPool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}

// LoadClientTLS creates a TLS config for the client (active/host) side.
// certFile and keyFile are optional; if provided, enables mutual TLS.
// caFile is optional; if provided, verifies the server certificate against this CA.
func LoadClientTLS(certFile, keyFile, caFile string) (*tls.Config, error) {
	cfg := DefaultTLSConfig()

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("security: load client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("security: read CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("security: failed to parse CA cert")
		}
		cfg.RootCAs = caPool
	}

	return cfg, nil
}

// GenerateTestTLSConfigs generates a matching server + client TLS config pair
// for testing. Uses an in-memory self-signed CA. Do NOT use in production.
func GenerateTestTLSConfigs() (server *tls.Config, client *tls.Config, err error) {
	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, err
	}
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// Generate server cert
	serverCert, err := generateCert(caCert, caKey, "server")
	if err != nil {
		return nil, nil, err
	}

	// Generate client cert
	clientCert, err := generateCert(caCert, caKey, "client")
	if err != nil {
		return nil, nil, err
	}

	server = DefaultTLSConfig()
	server.Certificates = []tls.Certificate{*serverCert}
	server.ClientCAs = caPool
	server.ClientAuth = tls.RequireAndVerifyClientCert

	client = DefaultTLSConfig()
	client.Certificates = []tls.Certificate{*clientCert}
	client.RootCAs = caPool
	client.ServerName = "localhost"

	return server, client, nil
}

func generateCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, name string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
