package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"phosphornet/internal/config"
	"phosphornet/internal/identity"
)

func tlsConfigForNode(cfg config.NodeConfig) (*tls.Config, error) {
	if !cfg.TLS.Enabled {
		return nil, nil
	}

	passport := &identity.Passport{
		DisplayName: cfg.Name,
		PublicKey:   cfg.NodeID,
		PrivateKey:  cfg.PrivateKey,
	}
	if err := passport.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node identity for tls: %w", err)
	}

	publicKey, err := passport.PublicKeyBytes()
	if err != nil {
		return nil, fmt.Errorf("load node public key for tls: %w", err)
	}
	privateKey, err := passport.PrivateKeyBytes()
	if err != nil {
		return nil, fmt.Errorf("load node private key for tls: %w", err)
	}

	certificate, err := selfSignedNodeCertificate(cfg.Name, publicKey, privateKey)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func selfSignedNodeCertificate(stationName string, publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey) (tls.Certificate, error) {
	template := &x509.Certificate{
		SerialNumber: serialForPublicKey(publicKey),
		Subject: pkix.Name{
			CommonName:   strings.TrimSpace(stationName),
			Organization: []string{"PhosphorNet"},
		},
		NotBefore:             time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}
	if template.Subject.CommonName == "" {
		template.Subject.CommonName = "PhosphorNet Station"
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create self-signed tls certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal tls private key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load tls key pair: %w", err)
	}
	return pair, nil
}

func serialForPublicKey(publicKey ed25519.PublicKey) *big.Int {
	sum := sha256.Sum256(publicKey)
	serial := new(big.Int).SetBytes(sum[:20])
	if serial.Sign() <= 0 {
		return big.NewInt(1)
	}
	return serial
}
