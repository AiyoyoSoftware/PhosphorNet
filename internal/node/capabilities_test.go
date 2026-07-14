package node

import (
	"context"
	"path/filepath"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func TestAdminDoorAccessDoesNotGrantAdminOpsWithoutCapability(t *testing.T) {
	session := &sessionState{publicKey: "ed25519:admin", role: "admin"}
	door := runtime.DoorManifest{ID: "ops", Access: accessAdmin}
	response := runtime.DoorResponse{
		AdminOps: []protocol.AdminOp{{Op: "set_maintenance", Maintenance: boolPtr(true)}},
	}

	if err := validateResponseCapabilities(session, door, response); err == nil {
		t.Fatal("validateResponseCapabilities() error = nil, want missing admin capability rejection")
	}
}

func TestModerationAdminOpsRequireModerationCapability(t *testing.T) {
	session := &sessionState{publicKey: "ed25519:admin", role: "admin"}
	response := runtime.DoorResponse{
		AdminOps: []protocol.AdminOp{{Op: "ban_key", PublicKey: "ed25519:bad", Reason: "spam"}},
	}

	if err := validateResponseCapabilities(session, runtime.DoorManifest{ID: "admin"}, response); err == nil {
		t.Fatal("validateResponseCapabilities() error = nil, want missing moderation capability rejection")
	}
	if err := validateResponseCapabilities(session, runtime.DoorManifest{
		ID:           "admin",
		Capabilities: []string{runtime.CapabilityAdminModerateUsers},
	}, response); err != nil {
		t.Fatalf("validateResponseCapabilities() error = %v, want moderation capability accepted", err)
	}
}

func TestGlobalStateWriteRequiresRoleAndCapability(t *testing.T) {
	door := runtime.DoorManifest{
		ID:           "admin-ish",
		Capabilities: []string{runtime.CapabilityStateGlobalWrite},
	}
	response := runtime.DoorResponse{
		StateOps: []protocol.StateOp{{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "motd", Value: "hi"}},
	}

	if err := validateResponseCapabilities(&sessionState{role: "member"}, door, response); err == nil {
		t.Fatal("validateResponseCapabilities() error = nil, want role rejection")
	}

	door.Capabilities = nil
	if err := validateResponseCapabilities(&sessionState{role: "admin"}, door, response); err == nil {
		t.Fatal("validateResponseCapabilities() error = nil, want capability rejection")
	}
}

func TestActionEffectsRequireRuleSpecificCapability(t *testing.T) {
	response := runtime.DoorResponse{Actions: []protocol.ActionEffect{{RequestID: "request-1", RuleID: "status"}}}
	door := runtime.DoorManifest{ID: "tools"}
	if err := validateResponseCapabilities(&sessionState{role: "member"}, door, response); err == nil {
		t.Fatal("validateResponseCapabilities() error = nil, want action capability rejection")
	}
	door.Capabilities = []string{runtime.ActionCapability("status")}
	if err := validateResponseCapabilities(&sessionState{role: "member"}, door, response); err != nil {
		t.Fatalf("validateResponseCapabilities() error = %v, want action capability accepted", err)
	}
}

func TestLoadStationPolicyMigratesLegacyAdminGlobalState(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	err = store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "roles", Value: map[string]any{"ed25519:user": "moderator"}},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}

	server := newServer(config.DefaultNodeConfig(), nil, store)
	if got := server.roleForPublicKey(ctx, "ed25519:user"); got != "moderator" {
		t.Fatalf("roleForPublicKey() = %q, want migrated moderator role", got)
	}
	nodeState, err := store.LoadNodeState(ctx, stationPolicyNodeStateKey)
	if err != nil {
		t.Fatalf("LoadNodeState() error = %v", err)
	}
	if len(nodeState) == 0 {
		t.Fatal("node station policy was not saved during legacy migration")
	}
}

func boolPtr(value bool) *bool {
	return &value
}
