package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type integrationHarness struct {
	server     *Server
	httpServer *httptest.Server
	wsURL      string
	store      *storage.Store
	node       *identity.Passport
}

func TestIntegrationCompatibleClientAuthRendersLobbyAndPersistsState(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	incompatiblePassport, err := identity.Generate("old-client")
	if err != nil {
		t.Fatalf("Generate(old-client) error = %v", err)
	}
	conn, _, err := websocket.Dial(ctx, h.wsURL, nil)
	if err != nil {
		t.Fatalf("Dial(incompatible) error = %v", err)
	}
	hello := protocol.DefaultHello(incompatiblePassport.PublicKey)
	hello.SupportedComponents = []string{"screen"}
	if err := wsjson.Write(ctx, conn, hello); err != nil {
		t.Fatalf("write incompatible hello: %v", err)
	}
	var errorMessage protocol.ErrorMessage
	if err := wsjson.Read(ctx, conn, &errorMessage); err != nil {
		t.Fatalf("read incompatible error: %v", err)
	}
	if errorMessage.Code != string(protocol.ErrorClientIncompatible) {
		t.Fatalf("incompatible error code = %q, want %q", errorMessage.Code, protocol.ErrorClientIncompatible)
	}
	conn.CloseNow()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, firstRender := connectIntegrationClient(t, ctx, h.wsURL, passport)
	if !uiTreeContainsText(firstRender.View, "MOTD: Default MOTD") {
		t.Fatalf("first lobby render = %#v, want default MOTD", firstRender.View)
	}
	if !uiTreeContainsText(firstRender.View, "Visits: 1") {
		t.Fatalf("first lobby render = %#v, want first persisted visit", firstRender.View)
	}
	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("close first connection: %v", err)
	}

	conn, secondRender := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()
	if !uiTreeContainsText(secondRender.View, "Visits: 2") {
		t.Fatalf("second lobby render = %#v, want visit count persisted through SQLite", secondRender.View)
	}
}

func TestIntegrationAdminDoorSettingRerendersActiveSessions(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationAdminDoor(t, doorsDir, "")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	memberPassport, err := identity.Generate("member")
	if err != nil {
		t.Fatalf("Generate(member) error = %v", err)
	}
	memberConn, memberRender := connectIntegrationClient(t, ctx, h.wsURL, memberPassport)
	defer memberConn.CloseNow()
	if !uiTreeContainsText(memberRender.View, "MOTD: Default MOTD") {
		t.Fatalf("member initial render = %#v, want default MOTD", memberRender.View)
	}

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	if !uiTreeContainsText(adminRender.View, "Admin Console") {
		t.Fatalf("admin render = %#v, want admin console", adminRender.View)
	}

	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "set-motd", "set_motd")
	memberUpdate := readIntegrationRender(t, ctx, memberConn)
	if !uiTreeContainsText(memberUpdate.View, "MOTD: Admin MOTD") {
		t.Fatalf("member rerender = %#v, want admin-updated MOTD", memberUpdate.View)
	}

	policy := h.server.loadStationPolicy(ctx)
	if policy.DoorSettings["lobby"]["motd"] != "Admin MOTD" {
		t.Fatalf("persisted lobby motd = %#v, want Admin MOTD", policy.DoorSettings["lobby"]["motd"])
	}
}

func TestIntegrationModerationBanDisconnectsAndBlocksReconnect(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	targetPassport, err := identity.Generate("target")
	if err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	writeIntegrationAdminDoor(t, doorsDir, targetPassport.PublicKey)

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	targetConn, _ := connectIntegrationClient(t, ctx, h.wsURL, targetPassport)
	defer targetConn.CloseNow()

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)

	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "ban-target", "ban_target")

	targetError := readIntegrationError(t, ctx, targetConn)
	if targetError.Code != string(protocol.ErrorAuth) || !strings.Contains(targetError.Message, "station access revoked") {
		t.Fatalf("target error = %#v, want auth revocation", targetError)
	}

	reconnect, _, err := websocket.Dial(ctx, h.wsURL, nil)
	if err != nil {
		t.Fatalf("Dial(reconnect) error = %v", err)
	}
	defer reconnect.CloseNow()
	if err := wsjson.Write(ctx, reconnect, protocol.DefaultHello(targetPassport.PublicKey)); err != nil {
		t.Fatalf("write reconnect hello: %v", err)
	}
	var challenge protocol.ChallengeMessage
	if err := wsjson.Read(ctx, reconnect, &challenge); err != nil {
		t.Fatalf("read reconnect challenge: %v", err)
	}
	payload := identity.LoginPayload{
		Purpose:         "phosphornet.login.v1",
		NodeID:          challenge.Payload.NodeID,
		ClientPublicKey: targetPassport.PublicKey,
		Nonce:           challenge.Payload.Nonce,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
	signature, err := identity.SignLogin(targetPassport, payload)
	if err != nil {
		t.Fatalf("SignLogin(reconnect) error = %v", err)
	}
	if err := wsjson.Write(ctx, reconnect, protocol.AuthMessage{Type: protocol.TypeAuth, Payload: payload, Signature: signature}); err != nil {
		t.Fatalf("write reconnect auth: %v", err)
	}
	var denied protocol.AuthDeniedMessage
	if err := wsjson.Read(ctx, reconnect, &denied); err != nil {
		t.Fatalf("read reconnect denial: %v", err)
	}
	if denied.Type != protocol.TypeAuthDenied || denied.Reason != "station access revoked" {
		t.Fatalf("reconnect response = %#v, want station access revoked", denied)
	}
}

func TestIntegrationAdminDoorPolicyRefreshesVisibilityAndDeniesDisabledDoor(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationSimpleDoor(t, doorsDir, "puzzle", "Puzzle")
	writeIntegrationAdminDoor(t, doorsDir, "", "disable_puzzle")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	memberPassport, err := identity.Generate("member")
	if err != nil {
		t.Fatalf("Generate(member) error = %v", err)
	}
	memberConn, _ := connectIntegrationClient(t, ctx, h.wsURL, memberPassport)
	defer memberConn.CloseNow()

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "disable-puzzle", "disable_puzzle")

	refreshedDoors := readIntegrationDoorList(t, ctx, memberConn)
	if doorListContains(refreshedDoors, "puzzle") {
		t.Fatalf("refreshed member door list = %#v, want puzzle removed after disable", refreshedDoors.Doors)
	}

	openDoor(t, ctx, memberConn, "puzzle")
	denied := readIntegrationError(t, ctx, memberConn)
	if denied.Code != string(protocol.ErrorRuntimeDeniedPolicy) {
		t.Fatalf("disabled door error = %#v, want runtime_denied_by_policy", denied)
	}
}

func TestIntegrationMutedUserWriteEventsRejectedButNavigationAllowed(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationSimpleDoor(t, doorsDir, "profile", "Profile")

	targetPassport, err := identity.Generate("target")
	if err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	writeIntegrationAdminDoor(t, doorsDir, targetPassport.PublicKey, "mute_target")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	targetConn, targetRender := connectIntegrationClient(t, ctx, h.wsURL, targetPassport)
	defer targetConn.CloseNow()

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "mute-target", "mute_target")

	_ = readIntegrationDoorList(t, ctx, targetConn)
	sendIntegrationAction(t, ctx, targetConn, targetRender.SessionID, "write-message", "write_message")
	mutedError := readIntegrationError(t, ctx, targetConn)
	if mutedError.Code != string(protocol.ErrorProtocol) || !strings.Contains(mutedError.Message, "user is muted") {
		t.Fatalf("muted write error = %#v, want protocol error for muted write", mutedError)
	}

	sendIntegrationAction(t, ctx, targetConn, targetRender.SessionID, "open-profile", "open_door:profile")
	profileRender := readIntegrationRender(t, ctx, targetConn)
	if !uiTreeContainsText(profileRender.View, "Profile") {
		t.Fatalf("muted navigation render = %#v, want profile door", profileRender.View)
	}
}

func TestIntegrationForgedEventTargetRejectedBeforeDoorUpdate(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, render := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	sendIntegrationAction(t, ctx, conn, render.SessionID, "not-in-render", "write_message")
	rejected := readIntegrationError(t, ctx, conn)
	if rejected.Code != string(protocol.ErrorProtocol) || !strings.Contains(rejected.Message, "not present in the active render") {
		t.Fatalf("forged event error = %#v, want render policy rejection", rejected)
	}
}

func TestIntegrationDoorCrashReturnsTypedRuntimeError(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationCrashDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, _ := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	openDoor(t, ctx, conn, "crash")
	crashed := readIntegrationError(t, ctx, conn)
	if crashed.Code != string(protocol.ErrorDoorCrashed) || !strings.Contains(crashed.Message, "integration boom") {
		t.Fatalf("crash error = %#v, want typed door_crashed error", crashed)
	}
}

func TestIntegrationInviteOnlyAdmissionAllowsAllowlistAndAdminsOnly(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	allowedPassport, err := identity.Generate("allowed")
	if err != nil {
		t.Fatalf("Generate(allowed) error = %v", err)
	}
	outsiderPassport, err := identity.Generate("outsider")
	if err != nil {
		t.Fatalf("Generate(outsider) error = %v", err)
	}
	h := newIntegrationHarnessWithConfig(t, doorsDir, []string{adminPassport.PublicKey}, func(cfg *config.NodeConfig) {
		cfg.Access.Mode = "invite_only"
		cfg.Access.Allowlist = []string{allowedPassport.PublicKey}
	})
	defer h.close()

	allowedConn, _ := connectIntegrationClient(t, ctx, h.wsURL, allowedPassport)
	defer allowedConn.CloseNow()
	adminConn, _ := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()

	outsiderConn, _, err := websocket.Dial(ctx, h.wsURL, nil)
	if err != nil {
		t.Fatalf("Dial(outsider) error = %v", err)
	}
	defer outsiderConn.CloseNow()
	denied := authenticateIntegrationDenied(t, ctx, outsiderConn, outsiderPassport)
	if denied.Reason != "station is invite-only" {
		t.Fatalf("outsider denial = %#v, want invite-only denial", denied)
	}
}

func TestIntegrationAdminRolePolicyRefreshesRoleGatedDoorAccess(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationSimpleDoorWithAccess(t, doorsDir, "vault", "Vault", "admin")

	memberPassport, err := identity.Generate("member")
	if err != nil {
		t.Fatalf("Generate(member) error = %v", err)
	}
	writeIntegrationAdminDoor(t, doorsDir, memberPassport.PublicKey, "grant_vault")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	memberConn, _ := connectIntegrationClient(t, ctx, h.wsURL, memberPassport)
	defer memberConn.CloseNow()
	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()

	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "grant-vault", "grant_vault")

	refreshedDoors := readIntegrationDoorList(t, ctx, memberConn)
	if !doorListContains(refreshedDoors, "vault") {
		t.Fatalf("refreshed member door list = %#v, want vault after role grant", refreshedDoors.Doors)
	}
	openDoor(t, ctx, memberConn, "vault")
	vaultRender := readIntegrationRender(t, ctx, memberConn)
	if !uiTreeContainsText(vaultRender.View, "Vault") {
		t.Fatalf("vault render = %#v, want vault after role policy update", vaultRender.View)
	}
}

func TestIntegrationProfileUpdateRerendersOtherPresenceViews(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationProfileUpdateDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	observerPassport, err := identity.Generate("observer")
	if err != nil {
		t.Fatalf("Generate(observer) error = %v", err)
	}
	observerConn, _ := connectIntegrationClient(t, ctx, h.wsURL, observerPassport)
	defer observerConn.CloseNow()

	actorPassport, err := identity.Generate("actor")
	if err != nil {
		t.Fatalf("Generate(actor) error = %v", err)
	}
	actorConn, _ := connectIntegrationClient(t, ctx, h.wsURL, actorPassport)
	defer actorConn.CloseNow()
	openDoor(t, ctx, actorConn, "profile")
	profileRender := readIntegrationRender(t, ctx, actorConn)
	sendIntegrationAction(t, ctx, actorConn, profileRender.SessionID, "save-profile", "save_profile")

	observerUpdate := readIntegrationRender(t, ctx, observerConn)
	if !uiTreeContainsText(observerUpdate.View, "Online: Ada") {
		t.Fatalf("observer rerender = %#v, want updated profile presence", observerUpdate.View)
	}
}

func TestIntegrationBadRuntimeOutputReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationBadOutputDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, _ := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	openDoor(t, ctx, conn, "badui")
	badOutput := readIntegrationError(t, ctx, conn)
	if badOutput.Code != string(protocol.ErrorRuntimeBadOutput) || !strings.Contains(badOutput.Message, "unsupported component") {
		t.Fatalf("bad output error = %#v, want typed runtime_bad_output", badOutput)
	}
}

func TestIntegrationAdminReloadManifestsPublishesNewDoorList(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationAdminDoor(t, doorsDir, "", "reload_manifests")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	memberPassport, err := identity.Generate("member")
	if err != nil {
		t.Fatalf("Generate(member) error = %v", err)
	}
	memberConn, _ := connectIntegrationClient(t, ctx, h.wsURL, memberPassport)
	defer memberConn.CloseNow()

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	writeIntegrationSimpleDoor(t, doorsDir, "news", "News")

	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "reload-manifests", "reload_manifests")

	refreshedDoors := readIntegrationDoorList(t, ctx, memberConn)
	if !doorListContains(refreshedDoors, "news") {
		t.Fatalf("refreshed member door list = %#v, want reloaded news door", refreshedDoors.Doors)
	}
}

func TestIntegrationRoomNotificationFanout(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationNotifyDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	firstPassport, err := identity.Generate("first")
	if err != nil {
		t.Fatalf("Generate(first) error = %v", err)
	}
	firstConn, _ := connectIntegrationClient(t, ctx, h.wsURL, firstPassport)
	defer firstConn.CloseNow()
	openDoor(t, ctx, firstConn, "notify")
	firstRender := readIntegrationRender(t, ctx, firstConn)

	secondPassport, err := identity.Generate("second")
	if err != nil {
		t.Fatalf("Generate(second) error = %v", err)
	}
	secondConn, _ := connectIntegrationClient(t, ctx, h.wsURL, secondPassport)
	defer secondConn.CloseNow()
	openDoor(t, ctx, secondConn, "notify")
	_ = readIntegrationRender(t, ctx, secondConn)

	sendIntegrationAction(t, ctx, firstConn, firstRender.SessionID, "ping-room", "ping_room")
	notify := readIntegrationNotify(t, ctx, secondConn)
	if notify.Message != "room ping" || notify.Level != "info" {
		t.Fatalf("room notify = %#v, want info room ping", notify)
	}
}

func TestIntegrationBroadcastRerendersRoomStateForPeers(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationBroadcastDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	firstPassport, err := identity.Generate("first")
	if err != nil {
		t.Fatalf("Generate(first) error = %v", err)
	}
	firstConn, _ := connectIntegrationClient(t, ctx, h.wsURL, firstPassport)
	defer firstConn.CloseNow()
	openDoor(t, ctx, firstConn, "counter")
	firstRender := readIntegrationRender(t, ctx, firstConn)

	secondPassport, err := identity.Generate("second")
	if err != nil {
		t.Fatalf("Generate(second) error = %v", err)
	}
	secondConn, _ := connectIntegrationClient(t, ctx, h.wsURL, secondPassport)
	defer secondConn.CloseNow()
	openDoor(t, ctx, secondConn, "counter")
	_ = readIntegrationRender(t, ctx, secondConn)

	sendIntegrationAction(t, ctx, firstConn, firstRender.SessionID, "increment", "increment")
	secondUpdate := readIntegrationRender(t, ctx, secondConn)
	if !uiTreeContainsText(secondUpdate.View, "Count: 1") {
		t.Fatalf("peer counter render = %#v, want broadcast rerender with room count", secondUpdate.View)
	}
}

func TestIntegrationPerUserEventRateLimit(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	targetPassport, err := identity.Generate("target")
	if err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	writeIntegrationAdminDoor(t, doorsDir, targetPassport.PublicKey, "rate_target")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	targetConn, targetRender := connectIntegrationClient(t, ctx, h.wsURL, targetPassport)
	defer targetConn.CloseNow()
	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "rate-target", "rate_target")

	_ = readIntegrationDoorList(t, ctx, targetConn)
	sendIntegrationAction(t, ctx, targetConn, targetRender.SessionID, "write-message", "write_message")
	_ = readIntegrationRender(t, ctx, targetConn)
	sendIntegrationAction(t, ctx, targetConn, targetRender.SessionID, "write-message", "write_message")
	limited := readIntegrationError(t, ctx, targetConn)
	if limited.Code != string(protocol.ErrorProtocol) || !strings.Contains(limited.Message, "event rate limit exceeded") {
		t.Fatalf("rate limit error = %#v, want event rate limit rejection", limited)
	}
}

func TestIntegrationTransitionBudgetExhaustionReturnsProtocolError(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationTransitionChain(t, doorsDir, protocol.MaxTransitionsPerResponse+2)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, _ := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	openDoor(t, ctx, conn, "chain0")
	exhausted := readIntegrationError(t, ctx, conn)
	if exhausted.Code != string(protocol.ErrorProtocol) || !strings.Contains(exhausted.Message, "transition budget exhausted") {
		t.Fatalf("transition error = %#v, want budget exhaustion", exhausted)
	}
}

func TestIntegrationStateWriteRequiresManifestCapability(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationStateWriteDeniedDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, _ := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	openDoor(t, ctx, conn, "writer")
	render := readIntegrationRender(t, ctx, conn)
	sendIntegrationAction(t, ctx, conn, render.SessionID, "write-state", "write_state")
	denied := readIntegrationError(t, ctx, conn)
	if denied.Code != string(protocol.ErrorRuntimeDeniedPolicy) || !strings.Contains(denied.Message, "state:user:write") {
		t.Fatalf("state write error = %#v, want missing state:user:write rejection", denied)
	}
}

func TestIntegrationCaptureKeysDoorReceivesKeyEvents(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationKeyCaptureDoor(t, doorsDir)

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate(traveler) error = %v", err)
	}
	conn, _ := connectIntegrationClient(t, ctx, h.wsURL, passport)
	defer conn.CloseNow()

	openDoor(t, ctx, conn, "keys")
	render := readIntegrationRender(t, ctx, conn)
	sendIntegrationKey(t, ctx, conn, render.SessionID, "x")
	updated := readIntegrationRender(t, ctx, conn)
	if !uiTreeContainsText(updated.View, "Keys: 1") {
		t.Fatalf("key capture render = %#v, want key count update", updated.View)
	}
}

func TestIntegrationInviteOnlyDoorAllowlistAcceptsFingerprint(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)

	allowedPassport, err := identity.Generate("allowed")
	if err != nil {
		t.Fatalf("Generate(allowed) error = %v", err)
	}
	writeIntegrationInviteOnlyDoor(t, doorsDir, identity.Fingerprint(allowedPassport.PublicKey))

	h := newIntegrationHarness(t, doorsDir, nil)
	defer h.close()

	allowedConn, _ := connectIntegrationClient(t, ctx, h.wsURL, allowedPassport)
	defer allowedConn.CloseNow()
	openDoor(t, ctx, allowedConn, "secret")
	secretRender := readIntegrationRender(t, ctx, allowedConn)
	if !uiTreeContainsText(secretRender.View, "Secret") {
		t.Fatalf("allowlisted render = %#v, want secret door", secretRender.View)
	}

	outsiderPassport, err := identity.Generate("outsider")
	if err != nil {
		t.Fatalf("Generate(outsider) error = %v", err)
	}
	outsiderConn, _ := connectIntegrationClient(t, ctx, h.wsURL, outsiderPassport)
	defer outsiderConn.CloseNow()
	openDoor(t, ctx, outsiderConn, "secret")
	denied := readIntegrationError(t, ctx, outsiderConn)
	if denied.Code != string(protocol.ErrorRuntimeDeniedPolicy) {
		t.Fatalf("outsider secret error = %#v, want runtime_denied_by_policy", denied)
	}
}

func TestIntegrationMaintenanceAdminOpFlowsIntoRuntimeContext(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationAdminDoor(t, doorsDir, "", "set_maintenance")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	memberPassport, err := identity.Generate("member")
	if err != nil {
		t.Fatalf("Generate(member) error = %v", err)
	}
	memberConn, memberRender := connectIntegrationClient(t, ctx, h.wsURL, memberPassport)
	defer memberConn.CloseNow()
	if !uiTreeContainsText(memberRender.View, "Maintenance: false") {
		t.Fatalf("member initial render = %#v, want maintenance false", memberRender.View)
	}

	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "set-maintenance", "set_maintenance")

	_ = readIntegrationDoorList(t, ctx, memberConn)
	sendIntegrationAction(t, ctx, memberConn, memberRender.SessionID, "write-message", "write_message")
	updated := readIntegrationRender(t, ctx, memberConn)
	if !uiTreeContainsText(updated.View, "Maintenance: true") {
		t.Fatalf("member maintenance render = %#v, want maintenance true", updated.View)
	}
}

func TestIntegrationPerUserOpenDoorRateLimit(t *testing.T) {
	ctx := context.Background()
	doorsDir := t.TempDir()
	writeIntegrationLobbyDoor(t, doorsDir)
	writeIntegrationSimpleDoor(t, doorsDir, "profile", "Profile")

	targetPassport, err := identity.Generate("target")
	if err != nil {
		t.Fatalf("Generate(target) error = %v", err)
	}
	writeIntegrationAdminDoor(t, doorsDir, targetPassport.PublicKey, "open_rate_target")

	adminPassport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate(admin) error = %v", err)
	}
	h := newIntegrationHarness(t, doorsDir, []string{adminPassport.PublicKey})
	defer h.close()

	targetConn, _ := connectIntegrationClient(t, ctx, h.wsURL, targetPassport)
	defer targetConn.CloseNow()
	adminConn, adminRender := connectIntegrationClient(t, ctx, h.wsURL, adminPassport)
	defer adminConn.CloseNow()
	openDoor(t, ctx, adminConn, "admin")
	adminRender = readIntegrationRender(t, ctx, adminConn)
	sendIntegrationAction(t, ctx, adminConn, adminRender.SessionID, "open-rate-target", "open_rate_target")

	_ = readIntegrationDoorList(t, ctx, targetConn)
	openDoor(t, ctx, targetConn, "profile")
	_ = readIntegrationRender(t, ctx, targetConn)
	openDoor(t, ctx, targetConn, "lobby")
	limited := readIntegrationError(t, ctx, targetConn)
	if limited.Code != string(protocol.ErrorProtocol) || !strings.Contains(limited.Message, "open_door rate limit exceeded") {
		t.Fatalf("open rate limit error = %#v, want open_door rate limit rejection", limited)
	}
}

func newIntegrationHarness(t *testing.T, doorsDir string, admins []string) integrationHarness {
	return newIntegrationHarnessWithConfig(t, doorsDir, admins, nil)
}

func newIntegrationHarnessWithConfig(t *testing.T, doorsDir string, admins []string, configure func(*config.NodeConfig)) integrationHarness {
	t.Helper()

	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	nodePassport, err := identity.Generate("node")
	if err != nil {
		store.Close()
		t.Fatalf("Generate(node) error = %v", err)
	}
	cfg := config.DefaultNodeConfig()
	cfg.Name = "integrationbox"
	cfg.NodeID = nodePassport.PublicKey
	cfg.PrivateKey = nodePassport.PrivateKey
	cfg.DoorsDir = doorsDir
	cfg.Database = store.Path()
	cfg.Access.Admins = append([]string{}, admins...)
	if configure != nil {
		configure(&cfg)
	}

	manifests, err := runtime.LoadDoorManifests(doorsDir)
	if err != nil {
		store.Close()
		t.Fatalf("LoadDoorManifests() error = %v", err)
	}
	server := newServer(cfg, manifests, store)
	httpServer := httptest.NewServer(http.HandlerFunc(server.handleWS))
	return integrationHarness{
		server:     server,
		httpServer: httpServer,
		wsURL:      "ws" + strings.TrimPrefix(httpServer.URL, "http"),
		store:      store,
		node:       nodePassport,
	}
}

func (h integrationHarness) close() {
	h.httpServer.Close()
	h.store.Close()
}

func connectIntegrationClient(t *testing.T, ctx context.Context, url string, passport *identity.Passport) (*websocket.Conn, protocol.RenderMessage) {
	t.Helper()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if err := wsjson.Write(ctx, conn, protocol.DefaultHello(passport.PublicKey)); err != nil {
		conn.CloseNow()
		t.Fatalf("write hello: %v", err)
	}

	var challenge protocol.ChallengeMessage
	if err := wsjson.Read(ctx, conn, &challenge); err != nil {
		conn.CloseNow()
		t.Fatalf("read challenge: %v", err)
	}
	if err := identity.VerifyNodeChallenge(challenge.Payload, challenge.Signature); err != nil {
		conn.CloseNow()
		t.Fatalf("VerifyNodeChallenge() error = %v", err)
	}
	if challenge.Payload.ClientPublicKey != passport.PublicKey {
		conn.CloseNow()
		t.Fatalf("challenge client public key = %q, want %q", challenge.Payload.ClientPublicKey, passport.PublicKey)
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
		conn.CloseNow()
		t.Fatalf("SignLogin() error = %v", err)
	}
	if err := wsjson.Write(ctx, conn, protocol.AuthMessage{Type: protocol.TypeAuth, Payload: payload, Signature: signature}); err != nil {
		conn.CloseNow()
		t.Fatalf("write auth: %v", err)
	}

	var authOK protocol.AuthOKMessage
	if err := wsjson.Read(ctx, conn, &authOK); err != nil {
		conn.CloseNow()
		t.Fatalf("read auth ok: %v", err)
	}
	if authOK.Type != protocol.TypeAuthOK {
		conn.CloseNow()
		t.Fatalf("auth response = %#v, want auth_ok", authOK)
	}

	var doorList protocol.DoorListMessage
	if err := wsjson.Read(ctx, conn, &doorList); err != nil {
		conn.CloseNow()
		t.Fatalf("read door list: %v", err)
	}
	if doorList.Type != protocol.TypeDoorList {
		conn.CloseNow()
		t.Fatalf("door list response = %#v, want door_list", doorList)
	}

	return conn, readIntegrationRender(t, ctx, conn)
}

func authenticateIntegrationDenied(t *testing.T, ctx context.Context, conn *websocket.Conn, passport *identity.Passport) protocol.AuthDeniedMessage {
	t.Helper()
	if err := wsjson.Write(ctx, conn, protocol.DefaultHello(passport.PublicKey)); err != nil {
		t.Fatalf("write denied hello: %v", err)
	}
	var challenge protocol.ChallengeMessage
	if err := wsjson.Read(ctx, conn, &challenge); err != nil {
		t.Fatalf("read denied challenge: %v", err)
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
		t.Fatalf("SignLogin(denied) error = %v", err)
	}
	if err := wsjson.Write(ctx, conn, protocol.AuthMessage{Type: protocol.TypeAuth, Payload: payload, Signature: signature}); err != nil {
		t.Fatalf("write denied auth: %v", err)
	}
	var denied protocol.AuthDeniedMessage
	if err := wsjson.Read(ctx, conn, &denied); err != nil {
		t.Fatalf("read auth denial: %v", err)
	}
	if denied.Type != protocol.TypeAuthDenied {
		t.Fatalf("denied response = %#v, want auth_denied", denied)
	}
	return denied
}

func openDoor(t *testing.T, ctx context.Context, conn *websocket.Conn, doorID string) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, protocol.OpenDoorMessage{Type: protocol.TypeOpenDoor, DoorID: doorID}); err != nil {
		t.Fatalf("open door %q: %v", doorID, err)
	}
}

func sendIntegrationAction(t *testing.T, ctx context.Context, conn *websocket.Conn, sessionID, target, action string) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, protocol.EventMessage{
		Type:      protocol.TypeEvent,
		SessionID: sessionID,
		Event: protocol.UIEvent{
			Kind:   protocol.EventKindAction,
			Target: target,
			Action: action,
		},
	}); err != nil {
		t.Fatalf("send action %q: %v", action, err)
	}
}

func sendIntegrationKey(t *testing.T, ctx context.Context, conn *websocket.Conn, sessionID, key string) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, protocol.EventMessage{
		Type:      protocol.TypeEvent,
		SessionID: sessionID,
		Event: protocol.UIEvent{
			Kind: protocol.EventKindKey,
			Key:  key,
		},
	}); err != nil {
		t.Fatalf("send key %q: %v", key, err)
	}
}

func readIntegrationRender(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.RenderMessage {
	t.Helper()
	raw := readIntegrationRawMust(t, ctx, conn)
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

func readIntegrationError(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.ErrorMessage {
	t.Helper()
	raw := readIntegrationRawMust(t, ctx, conn)
	var errorMessage protocol.ErrorMessage
	if err := json.Unmarshal(raw, &errorMessage); err != nil {
		t.Fatalf("decode error message: %v", err)
	}
	if errorMessage.Type != protocol.TypeError {
		t.Fatalf("message type = %q, want error: %s", errorMessage.Type, string(raw))
	}
	return errorMessage
}

func readIntegrationNotify(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.NotifyMessage {
	t.Helper()
	raw := readIntegrationRawMust(t, ctx, conn)
	var notify protocol.NotifyMessage
	if err := json.Unmarshal(raw, &notify); err != nil {
		t.Fatalf("decode notify message: %v", err)
	}
	if notify.Type != protocol.TypeNotify {
		t.Fatalf("message type = %q, want notify: %s", notify.Type, string(raw))
	}
	return notify
}

func readIntegrationDoorList(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.DoorListMessage {
	t.Helper()
	raw := readIntegrationRawMust(t, ctx, conn)
	var doorList protocol.DoorListMessage
	if err := json.Unmarshal(raw, &doorList); err != nil {
		t.Fatalf("decode door list: %v", err)
	}
	if doorList.Type != protocol.TypeDoorList {
		t.Fatalf("message type = %q, want door_list: %s", doorList.Type, string(raw))
	}
	return doorList
}

func doorListContains(message protocol.DoorListMessage, id string) bool {
	for _, door := range message.Doors {
		if door.ID == id {
			return true
		}
	}
	return false
}

func readIntegrationRawMust(t *testing.T, ctx context.Context, conn *websocket.Conn) json.RawMessage {
	t.Helper()
	raw, err := readIntegrationRaw(ctx, conn)
	if err != nil {
		t.Fatalf("read raw message: %v", err)
	}
	return raw
}

func readIntegrationRaw(ctx context.Context, conn *websocket.Conn) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func writeIntegrationLobbyDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "lobby", `
id = "lobby"
name = "Lobby"
entry = "app.lua"
runtime = "lua"
capabilities = ["state:user:read", "state:user:write", "transition:open_door"]

[settings.motd]
type = "string"
label = "Message of the day"
default = "Default MOTD"
`, `
local ui = phosphornet.ui

local function visits(ctx)
  return tonumber(ctx.store:get("user", "visits", 0)) or 0
end

function view(ctx)
  local motd = "Default MOTD"
  if ctx.settings and ctx.settings.motd then
    motd = tostring(ctx.settings.motd)
  end
  local names = {}
  for _, user in ipairs((ctx.presence and ctx.presence.all_users) or {}) do
    if user.display_name and user.display_name ~= "" then
      table.insert(names, user.display_name)
    end
  end
  local online = "Online: guest"
  if #names > 0 then
    online = "Online: " .. table.concat(names, ", ")
  end
  return ui.screen({
    ui.header("Lobby"),
    ui.text("MOTD: " .. motd),
    ui.text("Visits: " .. tostring(visits(ctx))),
    ui.text("Maintenance: " .. tostring(ctx.node.maintenance_mode == true)),
    ui.text(online),
    ui.button("write-message", "Write", "write_message"),
    ui.button("open-profile", "Profile", "open_door:profile"),
  })
end

function update(ctx, event)
  if event.action == "write_message" then
    ctx.store:set("user", "writes", tonumber(ctx.store:get("user", "writes", 0)) + 1)
  elseif event.action == "open_door:profile" then
    ctx.effects:transition("open_door", "profile")
  end
  return view(ctx)
end

function on_join(ctx)
  ctx.store:set("user", "visits", visits(ctx) + 1)
  return view(ctx)
end
`)
}

func writeIntegrationAdminDoor(t *testing.T, doorsDir string, targetPublicKey string, modes ...string) {
	t.Helper()
	buttons := ""
	handlers := ""
	if targetPublicKey != "" {
		if hasIntegrationMode(modes, "ban_target") || len(modes) == 0 {
			buttons += `, ui.button("ban-target", "Ban Target", "ban_target")`
			handlers += `
  elseif event.action == "ban_target" then
    ctx.effects:admin_op({ op = "ban_key", public_key = "` + targetPublicKey + `", reason = "integration test" })
`
		}
		if hasIntegrationMode(modes, "mute_target") {
			buttons += `, ui.button("mute-target", "Mute Target", "mute_target")`
			handlers += `
  elseif event.action == "mute_target" then
    ctx.effects:admin_op({ op = "mute_key", public_key = "` + targetPublicKey + `", reason = "integration test" })
`
		}
	}
	if hasIntegrationMode(modes, "disable_puzzle") {
		buttons += `, ui.button("disable-puzzle", "Disable Puzzle", "disable_puzzle")`
		handlers += `
  elseif event.action == "disable_puzzle" then
    ctx.effects:admin_op({ op = "set_door_enabled", door_id = "puzzle", enabled = false })
`
	}
	if hasIntegrationMode(modes, "reload_manifests") {
		buttons += `, ui.button("reload-manifests", "Reload", "reload_manifests")`
		handlers += `
  elseif event.action == "reload_manifests" then
    ctx.effects:admin_op({ op = "reload_manifests" })
`
	}
	if hasIntegrationMode(modes, "set_maintenance") {
		buttons += `, ui.button("set-maintenance", "Maintenance", "set_maintenance")`
		handlers += `
  elseif event.action == "set_maintenance" then
    ctx.effects:admin_op({ op = "set_maintenance", maintenance = true })
`
	}
	if targetPublicKey != "" && hasIntegrationMode(modes, "rate_target") {
		buttons += `, ui.button("rate-target", "Rate Target", "rate_target")`
		handlers += `
  elseif event.action == "rate_target" then
    ctx.effects:admin_op({ op = "set_user_rate_limit", public_key = "` + targetPublicKey + `", events_per_minute = 1 })
`
	}
	if targetPublicKey != "" && hasIntegrationMode(modes, "open_rate_target") {
		buttons += `, ui.button("open-rate-target", "Open Rate Target", "open_rate_target")`
		handlers += `
  elseif event.action == "open_rate_target" then
    ctx.effects:admin_op({ op = "set_user_rate_limit", public_key = "` + targetPublicKey + `", opens_per_minute = 1 })
`
	}
	if targetPublicKey != "" && hasIntegrationMode(modes, "grant_vault") {
		buttons += `, ui.button("grant-vault", "Grant Vault", "grant_vault")`
		handlers += `
  elseif event.action == "grant_vault" then
    ctx.effects:admin_op({ op = "set_door_roles", door_id = "vault", roles = { "moderator" } })
    ctx.effects:admin_op({ op = "set_user_role", public_key = "` + targetPublicKey + `", role = "moderator" })
`
	}
	writeTransitionTestDoor(t, doorsDir, "admin", `
id = "admin"
name = "Admin"
entry = "app.lua"
runtime = "lua"
access = "admin"
capabilities = ["admin:set_door_settings", "admin:moderate_users", "admin:set_door_policy", "admin:set_user_roles", "admin:reload_manifests", "admin:set_maintenance"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Admin Console"),
    ui.button("set-motd", "Set MOTD", "set_motd")`+buttons+`
  })
end

function update(ctx, event)
  if event.action == "set_motd" then
    ctx.effects:admin_op({ op = "set_door_setting", door_id = "lobby", setting_key = "motd", setting_value = "Admin MOTD" })
`+handlers+`  end
  return view(ctx)
end
`)
}

func hasIntegrationMode(modes []string, mode string) bool {
	for _, candidate := range modes {
		if candidate == mode {
			return true
		}
	}
	return false
}

func writeIntegrationSimpleDoor(t *testing.T, doorsDir, id, title string) {
	t.Helper()
	writeIntegrationSimpleDoorWithAccess(t, doorsDir, id, title, "")
}

func writeIntegrationSimpleDoorWithAccess(t *testing.T, doorsDir, id, title, access string) {
	t.Helper()
	accessLine := ""
	if access != "" {
		accessLine = "\naccess = \"" + access + "\""
	}
	writeTransitionTestDoor(t, doorsDir, id, `
id = "`+id+`"
name = "`+title+`"
entry = "app.lua"
runtime = "lua"`+accessLine+`
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("`+title+`"),
    ui.text("`+title+` door is active."),
  })
end
`)
}

func writeIntegrationProfileUpdateDoor(t *testing.T, doorsDir string) {
	t.Helper()
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
    ui.button("save-profile", "Save", "save_profile"),
  })
end

function update(ctx, event)
  if event.action == "save_profile" then
    ctx.effects:update_profile("Ada", "writes tests", "online")
  end
  return view(ctx)
end
`)
}

func writeIntegrationNotifyDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "notify", `
id = "notify"
name = "Notify"
entry = "app.lua"
runtime = "lua"
capabilities = ["notify:room"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Notify"),
    ui.button("ping-room", "Ping Room", "ping_room"),
  })
end

function update(ctx, event)
  if event.action == "ping_room" then
    ctx.effects:notify("room ping", "info", "room")
  end
  return view(ctx)
end
`)
}

func writeIntegrationBroadcastDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "counter", `
id = "counter"
name = "Counter"
entry = "app.lua"
runtime = "lua"
capabilities = ["state:room:read", "state:room:write", "broadcast:room"]
`, `
local ui = phosphornet.ui

local function count(ctx)
  return tonumber(ctx.store:get("room", "count", 0)) or 0
end

function view(ctx)
  return ui.screen({
    ui.header("Counter"),
    ui.text("Count: " .. tostring(count(ctx))),
    ui.button("increment", "Increment", "increment"),
  })
end

function update(ctx, event)
  if event.action == "increment" then
    ctx.store:set("room", "count", count(ctx) + 1)
    ctx.effects:broadcast({ kind = "focus", target = "increment" }, "room")
  end
  return view(ctx)
end
`)
}

func writeIntegrationStateWriteDeniedDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "writer", `
id = "writer"
name = "Writer"
entry = "app.lua"
runtime = "lua"
capabilities = ["state:user:read"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Writer"),
    ui.button("write-state", "Write State", "write_state"),
  })
end

function update(ctx, event)
  if event.action == "write_state" then
    ctx.store:set("user", "blocked", "yes")
  end
  return view(ctx)
end
`)
}

func writeIntegrationKeyCaptureDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "keys", `
id = "keys"
name = "Keys"
entry = "app.lua"
runtime = "lua"
capabilities = ["capture_keys", "state:user:read", "state:user:write"]
`, `
local ui = phosphornet.ui

local function key_count(ctx)
  return tonumber(ctx.store:get("user", "keys", 0)) or 0
end

function view(ctx)
  return {
    component = "screen",
    capture_keys = true,
    children = {
      ui.header("Keys"),
      ui.text("Keys: " .. tostring(key_count(ctx))),
    },
  }
end

function update(ctx, event)
  if event.kind == "key" then
    ctx.store:set("user", "keys", key_count(ctx) + 1)
  end
  return view(ctx)
end
`)
}

func writeIntegrationInviteOnlyDoor(t *testing.T, doorsDir, allowlistEntry string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "secret", `
id = "secret"
name = "Secret"
entry = "app.lua"
runtime = "lua"
access = "invite_only"
allowlist = ["`+allowlistEntry+`"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Secret"),
    ui.text("Secret door is active."),
  })
end
`)
}

func writeIntegrationCrashDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "crash", `
id = "crash"
name = "Crash"
entry = "app.lua"
runtime = "lua"
`, `
function view(ctx)
  error("integration boom")
end
`)
}

func writeIntegrationTransitionChain(t *testing.T, doorsDir string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		next := ""
		if i+1 < count {
			next = "chain" + string(rune('0'+i+1))
		}
		transition := ""
		if next != "" {
			transition = `
function on_join(ctx)
  ctx.effects:transition("open_door", "` + next + `")
  return view(ctx)
end
`
		}
		writeTransitionTestDoor(t, doorsDir, "chain"+string(rune('0'+i)), `
id = "chain`+string(rune('0'+i))+`"
name = "Chain `+string(rune('0'+i))+`"
entry = "app.lua"
runtime = "lua"
capabilities = ["transition:open_door"]
`, `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("Chain `+string(rune('0'+i))+`"),
  })
end
`+transition)
	}
}

func writeIntegrationBadOutputDoor(t *testing.T, doorsDir string) {
	t.Helper()
	writeTransitionTestDoor(t, doorsDir, "badui", `
id = "badui"
name = "Bad UI"
entry = "app.lua"
runtime = "lua"
`, `
function view(ctx)
  return { component = "screen", children = { { component = "blink", text = "bad" } } }
end
`)
}
