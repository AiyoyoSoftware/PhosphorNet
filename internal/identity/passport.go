package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

type Passport struct {
	DisplayName   string `toml:"display_name"`
	PublicKey     string `toml:"public_key"`
	PrivateKey    string `toml:"private_key"`
	CreatedAt     string `toml:"created_at"`
	SchemaVersion int    `toml:"schema_version"`
}

type LoginPayload struct {
	Purpose         string `json:"purpose"`
	NodeID          string `json:"node_id"`
	ClientPublicKey string `json:"client_public_key"`
	Nonce           string `json:"nonce"`
	Timestamp       string `json:"timestamp"`
}

type NodeChallengePayload struct {
	Purpose         string `json:"purpose"`
	NodeID          string `json:"node_id"`
	NodeName        string `json:"node_name"`
	ClientPublicKey string `json:"client_public_key"`
	Nonce           string `json:"nonce"`
	Timestamp       string `json:"timestamp"`
}

func Generate(displayName string) (*Passport, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 keypair: %w", err)
	}

	return &Passport{
		DisplayName:   displayName,
		PublicKey:     EncodePublicKey(publicKey),
		PrivateKey:    base64.StdEncoding.EncodeToString(privateKey),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: 1,
	}, nil
}

func Load(path string) (*Passport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read passport: %w", err)
	}

	var passport Passport
	if err := toml.Unmarshal(data, &passport); err != nil {
		return nil, fmt.Errorf("parse passport: %w", err)
	}
	if err := passport.Validate(); err != nil {
		return nil, fmt.Errorf("invalid passport file %q: %w", path, err)
	}
	return &passport, nil
}

func Save(path string, passport *Passport) error {
	data, err := toml.Marshal(passport)
	if err != nil {
		return fmt.Errorf("marshal passport: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func (p *Passport) PublicKeyBytes() (ed25519.PublicKey, error) {
	return DecodePublicKey(p.PublicKey)
}

func (p *Passport) PrivateKeyBytes() (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(p.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	privateKey := ed25519.PrivateKey(raw)
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(privateKey))
	}
	return privateKey, nil
}

func (p *Passport) Fingerprint() string {
	return Fingerprint(p.PublicKey)
}

func (p *Passport) Validate() error {
	if strings.TrimSpace(p.PublicKey) == "" {
		return fmt.Errorf("missing public_key")
	}
	if strings.TrimSpace(p.PrivateKey) == "" {
		return fmt.Errorf("missing private_key")
	}
	if _, err := p.PublicKeyBytes(); err != nil {
		return fmt.Errorf("invalid public_key: %w", err)
	}
	if _, err := p.PrivateKeyBytes(); err != nil {
		return fmt.Errorf("invalid private_key: %w", err)
	}
	return nil
}

func EncodePublicKey(publicKey ed25519.PublicKey) string {
	return "ed25519:" + base64.StdEncoding.EncodeToString(publicKey)
}

func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw := strings.TrimPrefix(encoded, "ed25519:")
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: %d", len(decoded))
	}
	return ed25519.PublicKey(decoded), nil
}

func Fingerprint(encodedPublicKey string) string {
	sum := sha256.Sum256([]byte(encodedPublicKey))
	encoded := strings.TrimRight(base32.StdEncoding.EncodeToString(sum[:10]), "=")
	if len(encoded) < 12 {
		return encoded
	}
	return fmt.Sprintf("%s-%s-%s", encoded[:4], encoded[4:8], encoded[8:12])
}

func SignLogin(passport *Passport, payload LoginPayload) (string, error) {
	privateKey, err := passport.PrivateKeyBytes()
	if err != nil {
		return "", err
	}
	blob, err := canonicalPayload(payload)
	if err != nil {
		return "", fmt.Errorf("marshal login payload: %w", err)
	}
	signature := ed25519.Sign(privateKey, blob)
	return base64.StdEncoding.EncodeToString(signature), nil
}

func VerifyLogin(payload LoginPayload, signature string) error {
	publicKey, err := DecodePublicKey(payload.ClientPublicKey)
	if err != nil {
		return err
	}
	blob, err := canonicalJSON(payload)
	if err != nil {
		return fmt.Errorf("marshal login payload: %w", err)
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(publicKey, blob, rawSignature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func SignNodeChallenge(passport *Passport, payload NodeChallengePayload) (string, error) {
	privateKey, err := passport.PrivateKeyBytes()
	if err != nil {
		return "", err
	}
	blob, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("marshal node challenge payload: %w", err)
	}
	signature := ed25519.Sign(privateKey, blob)
	return base64.StdEncoding.EncodeToString(signature), nil
}

func VerifyNodeChallenge(payload NodeChallengePayload, signature string) error {
	publicKey, err := DecodePublicKey(payload.NodeID)
	if err != nil {
		return err
	}
	blob, err := canonicalJSON(payload)
	if err != nil {
		return fmt.Errorf("marshal node challenge payload: %w", err)
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(publicKey, blob, rawSignature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func canonicalPayload(payload LoginPayload) ([]byte, error) {
	return canonicalJSON(payload)
}

func canonicalJSON(payload any) ([]byte, error) {
	return json.Marshal(payload)
}
