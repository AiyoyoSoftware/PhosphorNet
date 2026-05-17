package node

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"phosphornet/internal/identity"
)

func randomNonce() (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read random nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(nonce), nil
}

func randomSessionID() (string, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read random session nonce: %w", err)
	}
	return "s-" + base64.RawURLEncoding.EncodeToString(nonce), nil
}

func stringsToUpper(value string) string {
	return strings.ToUpper(value)
}

func implicitRoomID(doorID string) string {
	return "door:" + doorID
}

func fingerprintForPublicKey(publicKey string) string {
	return identity.Fingerprint(publicKey)
}
