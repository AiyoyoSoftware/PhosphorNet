package identity

import "testing"

func TestSignAndVerifyLogin(t *testing.T) {
	passport, err := Generate("tester")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	payload := LoginPayload{
		Purpose:         "phosphornet.login.v1",
		NodeID:          "ed25519:node",
		ClientPublicKey: passport.PublicKey,
		Nonce:           "nonce",
		Timestamp:       "2026-05-03T12:00:00Z",
	}

	signature, err := SignLogin(passport, payload)
	if err != nil {
		t.Fatalf("SignLogin() error = %v", err)
	}
	if err := VerifyLogin(payload, signature); err != nil {
		t.Fatalf("VerifyLogin() error = %v", err)
	}
}

func TestSignAndVerifyNodeChallenge(t *testing.T) {
	passport, err := Generate("node")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	payload := NodeChallengePayload{
		Purpose:         "phosphornet.node_challenge.v1",
		NodeID:          passport.PublicKey,
		NodeName:        "localbox",
		ClientPublicKey: "ed25519:client",
		Nonce:           "nonce",
		Timestamp:       "2026-05-10T12:00:00Z",
	}

	signature, err := SignNodeChallenge(passport, payload)
	if err != nil {
		t.Fatalf("SignNodeChallenge() error = %v", err)
	}
	if err := VerifyNodeChallenge(payload, signature); err != nil {
		t.Fatalf("VerifyNodeChallenge() error = %v", err)
	}
}

func TestPassportValidateRejectsMissingPrivateKey(t *testing.T) {
	passport := &Passport{
		PublicKey: "ed25519:Zm9v",
	}
	if err := passport.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}
