package client

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"phosphornet/internal/identity"
	"phosphornet/internal/knownnodes"
	"phosphornet/internal/protocol"
)

func TestInitCommandCreatesAndReusesPassport(t *testing.T) {
	passportPath := filepath.Join(t.TempDir(), "passport.toml")
	cmd := newInitCommand()
	cmd.SetArgs([]string{"--passport", passportPath})
	var output bytes.Buffer
	cmd.SetOut(&output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command error = %v", err)
	}
	if !strings.Contains(output.String(), "created passport") {
		t.Fatalf("init output = %q, want created passport", output.String())
	}
	first, err := os.ReadFile(passportPath)
	if err != nil {
		t.Fatalf("ReadFile(passport) error = %v", err)
	}

	output.Reset()
	cmd = newInitCommand()
	cmd.SetArgs([]string{"--passport", passportPath})
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command reuse error = %v", err)
	}
	if !strings.Contains(output.String(), "using existing passport") {
		t.Fatalf("init reuse output = %q, want using existing passport", output.String())
	}
	second, err := os.ReadFile(passportPath)
	if err != nil {
		t.Fatalf("ReadFile(passport reuse) error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("init command changed existing passport, want reuse")
	}
}

func TestNormalizeWebSocketURLExpandsFriendlyAddresses(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "empty default",
			raw:  "",
			want: "wss://127.0.0.1:7707/ws",
		},
		{
			name: "bare host",
			raw:  "localhost",
			want: "wss://localhost:7707/ws",
		},
		{
			name: "bare host and port",
			raw:  "localhost:7711",
			want: "wss://localhost:7711/ws",
		},
		{
			name: "explicit scheme and port",
			raw:  "ws://localhost:7711",
			want: "ws://localhost:7711/ws",
		},
		{
			name: "explicit websocket path",
			raw:  "wss://station.example:9443/ws",
			want: "wss://station.example:9443/ws",
		},
		{
			name: "path without websocket suffix",
			raw:  "wss://station.example:9443/custom",
			want: "wss://station.example:9443/custom/ws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWebSocketURL(tt.raw)
			if err != nil {
				t.Fatalf("normalizeWebSocketURL(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeWebSocketURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestConnectCommandRejectsAddressArgumentAndDeprecatedFlagTogether(t *testing.T) {
	cmd := newConnectCommand()
	cmd.SetArgs([]string{"localhost", "--addr", "wss://127.0.0.1:7707/ws"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("connect command error = nil, want conflict")
	}
	if !strings.Contains(err.Error(), "use either positional address or --addr") {
		t.Fatalf("connect command error = %q, want conflict message", err.Error())
	}
}

func TestPinNodeRejectsChangedKnownNodeKey(t *testing.T) {
	store := &knownnodes.KnownNodes{}
	path := filepath.Join(t.TempDir(), "known_nodes.toml")
	if err := pinNode("wss://127.0.0.1:7707/ws", "first-key", store, path, false); err != nil {
		t.Fatalf("pinNode() initial error = %v", err)
	}

	if err := pinNode("wss://127.0.0.1:7707/ws", "second-key", store, path, false); err == nil {
		t.Fatal("pinNode() changed key error = nil, want mismatch error")
	}
}

func TestPinNodeCanReplaceChangedKnownNodeKey(t *testing.T) {
	store := &knownnodes.KnownNodes{}
	path := filepath.Join(t.TempDir(), "known_nodes.toml")
	address := "wss://127.0.0.1:7707/ws"
	if err := pinNode(address, "first-key", store, path, false); err != nil {
		t.Fatalf("pinNode() initial error = %v", err)
	}

	if err := pinNode(address, "second-key", store, path, true); err != nil {
		t.Fatalf("pinNode() replace error = %v", err)
	}
	record, found := store.Find(address)
	if !found {
		t.Fatal("known node record not found")
	}
	if record.PublicKey != "second-key" {
		t.Fatalf("record.PublicKey = %q, want second-key", record.PublicKey)
	}
}

func TestVerifyServerChallengeRequiresValidNodeSignature(t *testing.T) {
	nodePassport, err := identity.Generate("node")
	if err != nil {
		t.Fatalf("Generate(node) error = %v", err)
	}

	payload := identity.NodeChallengePayload{
		Purpose:         "phosphornet.node_challenge.v1",
		NodeID:          nodePassport.PublicKey,
		NodeName:        "localbox",
		ClientPublicKey: "ed25519:client",
		Nonce:           "nonce",
		Timestamp:       "2026-05-10T12:00:00Z",
	}
	signature, err := identity.SignNodeChallenge(nodePassport, payload)
	if err != nil {
		t.Fatalf("SignNodeChallenge() error = %v", err)
	}

	if err := verifyServerChallenge(protocol.ChallengeMessage{
		Type:      protocol.TypeChallenge,
		Payload:   payload,
		Signature: signature,
	}, "ed25519:client"); err != nil {
		t.Fatalf("verifyServerChallenge() error = %v", err)
	}

	if err := verifyServerChallenge(protocol.ChallengeMessage{
		Type:      protocol.TypeChallenge,
		Payload:   payload,
		Signature: "bad-signature",
	}, "ed25519:client"); err == nil {
		t.Fatal("verifyServerChallenge() error = nil, want invalid signature failure")
	}
}

func TestBuildTrustSummarySeparatesSelfSignedTLSFromStationIdentity(t *testing.T) {
	nodePassport, err := identity.Generate("node")
	if err != nil {
		t.Fatalf("Generate(node) error = %v", err)
	}
	resp := &http.Response{TLS: &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		PeerCertificates: []*x509.Certificate{testSelfSignedCertificate(t)},
	}}

	summary, err := buildTrustSummary("wss://127.0.0.1:7707/ws", resp, nodePassport.PublicKey, "localbox", &knownnodes.KnownNodes{}, false)
	if err != nil {
		t.Fatalf("buildTrustSummary() error = %v", err)
	}

	if !summary.RequiresPrompt {
		t.Fatal("summary.RequiresPrompt = false, want first connection prompt")
	}
	if summary.Transport != "encrypted (TLS 1.3)" {
		t.Fatalf("summary.Transport = %q", summary.Transport)
	}
	if summary.Certificate != "self-signed station certificate" {
		t.Fatalf("summary.Certificate = %q", summary.Certificate)
	}
	if !strings.Contains(summary.HostnameVerification, "name matches 127.0.0.1") {
		t.Fatalf("summary.HostnameVerification = %q", summary.HostnameVerification)
	}
	if !strings.Contains(summary.StationIdentity, "new Ed25519 station identity") {
		t.Fatalf("summary.StationIdentity = %q", summary.StationIdentity)
	}
	if summary.StationName != "localbox" {
		t.Fatalf("summary.StationName = %q, want localbox", summary.StationName)
	}
}

func TestBuildTrustSummaryRejectsChangedKnownStationIdentity(t *testing.T) {
	oldNode, err := identity.Generate("old")
	if err != nil {
		t.Fatalf("Generate(old) error = %v", err)
	}
	newNode, err := identity.Generate("new")
	if err != nil {
		t.Fatalf("Generate(new) error = %v", err)
	}
	store := &knownnodes.KnownNodes{Nodes: []knownnodes.Record{{
		Address:   "wss://example.test/ws",
		PublicKey: oldNode.PublicKey,
		Name:      identity.Fingerprint(oldNode.PublicKey),
	}}}

	_, err = buildTrustSummary("wss://example.test/ws", nil, newNode.PublicKey, "localbox", store, false)
	if err == nil {
		t.Fatal("buildTrustSummary() error = nil, want changed identity failure")
	}
	message := err.Error()
	if !strings.Contains(message, "Pinned station identity") || !strings.Contains(message, "Presented station identity") {
		t.Fatalf("changed identity error did not explain both keys:\n%s", message)
	}
}

func TestTrustPromptTUIRequiresExplicitAcceptance(t *testing.T) {
	summary := trustSummary{
		Address:              "wss://127.0.0.1:7707/ws",
		Transport:            "encrypted (TLS 1.3)",
		Certificate:          "self-signed station certificate",
		HostnameVerification: "name matches 127.0.0.1, but issuer is not WebPKI trusted",
		StationName:          "localbox",
		StationIdentity:      "new Ed25519 station identity (ABCD-EFGH-IJKL)",
		StationPublicKey:     "ed25519:test",
		RequiresPrompt:       true,
	}

	model := newTrustPromptModel(summary)
	view := model.View()
	for _, want := range []string{"PhosphorNet First Connection Trust", "Transport", "Certificate", "Hostname", "Station Identity", "localbox", "Trust + Pin"} {
		if !strings.Contains(view, want) {
			t.Fatalf("trust TUI missing %q:\n%s", want, view)
		}
	}

	acceptedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	accepted := acceptedModel.(trustPromptModel)
	if !accepted.decided || !accepted.accepted {
		t.Fatalf("accepted model = %#v, want decided accepted", accepted)
	}

	rejectedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rejected := rejectedModel.(trustPromptModel)
	if !rejected.decided || rejected.accepted {
		t.Fatalf("rejected model = %#v, want decided rejected", rejected)
	}

	cancelSelectedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	cancelSelected := cancelSelectedModel.(trustPromptModel)
	enterRejectedModel, _ := cancelSelected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	enterRejected := enterRejectedModel.(trustPromptModel)
	if !enterRejected.decided || enterRejected.accepted {
		t.Fatalf("enterRejected model = %#v, want decided rejected", enterRejected)
	}
}

func testSelfSignedCertificate(t *testing.T) *x509.Certificate {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "PhosphorNet Station",
			Organization: []string{"PhosphorNet"},
		},
		NotBefore:             time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	return certificate
}
