package node

import (
	"crypto/ed25519"
	"crypto/x509"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/identity"
)

func TestTLSConfigForNodeUsesNodeEd25519Identity(t *testing.T) {
	passport, err := identity.Generate("localbox")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	cfg := config.DefaultNodeConfig()
	cfg.Name = "localbox"
	cfg.NodeID = passport.PublicKey
	cfg.PrivateKey = passport.PrivateKey

	tlsCfg, err := tlsConfigForNode(cfg)
	if err != nil {
		t.Fatalf("tlsConfigForNode() error = %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("tlsConfigForNode() = nil, want config")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Fatalf("len(tlsCfg.Certificates) = %d, want 1", len(tlsCfg.Certificates))
	}

	cert := tlsCfg.Certificates[0]
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if leaf.Subject.CommonName != "localbox" {
		t.Fatalf("leaf.Subject.CommonName = %q, want localbox", leaf.Subject.CommonName)
	}
	if got, ok := leaf.PublicKey.(ed25519.PublicKey); !ok {
		t.Fatalf("leaf.PublicKey type = %T, want ed25519.PublicKey", leaf.PublicKey)
	} else if string(got) != string(cert.PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey)) {
		t.Fatal("certificate public key does not match the private key")
	}
}
