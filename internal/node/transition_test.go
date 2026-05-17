package node

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/config"
	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func TestDoorTransitionDoesNotOverwriteTargetRender(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeTransitionTestDoor(t, doorsDir, "lobby", `
id = "lobby"
name = "Lobby"
entry = "app.lua"
runtime = "lua"
capabilities = ["transition:open_door"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Lobby"),
    ui.button("open-profile", "Open Profile Door", "open_door:profile"),
  })
end

function update(ctx, event)
  if event.action == "open_door:profile" then
    ctx.effects.transition("open_door", "profile")
  end
  return view(ctx)
end
`)
	writeTransitionTestDoor(t, doorsDir, "profile", `
id = "profile"
name = "Profile"
entry = "app.lua"
runtime = "lua"
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Profile"),
    ui.text("Profile door is active."),
  })
end
`)

	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	nodePassport, err := identity.Generate("node")
	if err != nil {
		t.Fatalf("Generate(node) error = %v", err)
	}
	cfg := config.DefaultNodeConfig()
	cfg.Name = "testbox"
	cfg.NodeID = nodePassport.PublicKey
	cfg.PrivateKey = nodePassport.PrivateKey
	cfg.DoorsDir = doorsDir

	manifests, err := runtime.LoadDoorManifests(doorsDir)
	if err != nil {
		t.Fatalf("LoadDoorManifests() error = %v", err)
	}
	server := newServer(cfg, manifests, store)
	httpServer := httptest.NewServer(http.HandlerFunc(server.handleWS))
	defer httpServer.Close()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.CloseNow()

	clientPassport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	authOK := authenticateTransitionTestClient(t, ctx, conn, clientPassport)

	var doorList protocol.DoorListMessage
	if err := wsjson.Read(ctx, conn, &doorList); err != nil {
		t.Fatalf("read door list: %v", err)
	}
	initialRender := readTransitionTestRender(t, ctx, conn)
	if !uiTreeContainsText(initialRender.View, "Lobby") {
		t.Fatalf("initial render = %#v, want lobby", initialRender.View)
	}

	if err := wsjson.Write(ctx, conn, protocol.EventMessage{
		Type:      protocol.TypeEvent,
		SessionID: initialRender.SessionID,
		Event: protocol.UIEvent{
			Kind:   protocol.EventKindAction,
			Target: "open-profile",
			Action: "open_door:profile",
		},
	}); err != nil {
		t.Fatalf("write profile transition event: %v", err)
	}

	profileRender := readTransitionTestRender(t, ctx, conn)
	if !uiTreeContainsText(profileRender.View, "Profile") {
		t.Fatalf("transition render = %#v, want profile for %s", profileRender.View, authOK.Fingerprint)
	}

	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if extra, err := readTransitionTestRaw(shortCtx, conn); err == nil {
		t.Fatalf("unexpected extra message after profile render: %s", string(extra))
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read extra message error = %v, want deadline", err)
	}
}

func writeTransitionTestDoor(t *testing.T, doorsDir, id, manifest, app string) {
	t.Helper()
	dir := filepath.Join(doorsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", id, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s manifest) error = %v", id, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.lua"), []byte(strings.TrimSpace(app)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s app) error = %v", id, err)
	}
}

func authenticateTransitionTestClient(t *testing.T, ctx context.Context, conn *websocket.Conn, passport *identity.Passport) protocol.AuthOKMessage {
	t.Helper()
	if err := wsjson.Write(ctx, conn, protocol.DefaultHello(passport.PublicKey)); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var challenge protocol.ChallengeMessage
	if err := wsjson.Read(ctx, conn, &challenge); err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	payload := identity.LoginPayload{
		Purpose:         "phosphornet.login.v1",
		NodeID:          challenge.Payload.NodeID,
		ClientPublicKey: passport.PublicKey,
		Nonce:           challenge.Payload.Nonce,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
	signature, err := identity.SignLogin(passport, payload)
	if err != nil {
		t.Fatalf("SignLogin() error = %v", err)
	}
	if err := wsjson.Write(ctx, conn, protocol.AuthMessage{
		Type:      protocol.TypeAuth,
		Payload:   payload,
		Signature: signature,
	}); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	var authOK protocol.AuthOKMessage
	if err := wsjson.Read(ctx, conn, &authOK); err != nil {
		t.Fatalf("read auth ok: %v", err)
	}
	if authOK.Type != protocol.TypeAuthOK {
		t.Fatalf("auth response type = %q, want %q", authOK.Type, protocol.TypeAuthOK)
	}
	return authOK
}

func readTransitionTestRender(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.RenderMessage {
	t.Helper()
	raw := readTransitionTestRawMust(t, ctx, conn)
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Type != protocol.TypeRender {
		t.Fatalf("message type = %q, want render: %s", envelope.Type, string(raw))
	}
	var render protocol.RenderMessage
	if err := json.Unmarshal(raw, &render); err != nil {
		t.Fatalf("decode render: %v", err)
	}
	return render
}

func readTransitionTestRawMust(t *testing.T, ctx context.Context, conn *websocket.Conn) json.RawMessage {
	t.Helper()
	raw, err := readTransitionTestRaw(ctx, conn)
	if err != nil {
		t.Fatalf("read raw message: %v", err)
	}
	return raw
}

func readTransitionTestRaw(ctx context.Context, conn *websocket.Conn) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func uiTreeContainsText(node protocol.UINode, text string) bool {
	if node.Text == text || node.Title == text {
		return true
	}
	for _, child := range node.Children {
		if uiTreeContainsText(child, text) {
			return true
		}
	}
	return false
}
