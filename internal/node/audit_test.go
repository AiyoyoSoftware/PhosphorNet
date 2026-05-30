package node

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func TestAdminOpsWriteAuditEventsToDatabaseAndFile(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	var auditFile bytes.Buffer
	server := newServerWithOptions(config.DefaultNodeConfig(), nil, store, serverOptions{AuditLogFile: &auditFile})
	session := &sessionState{publicKey: "ed25519:admin", role: "admin"}
	door := runtime.DoorManifest{
		ID:           adminDoorID,
		Capabilities: []string{runtime.CapabilityAdminSetDoorPolicy},
	}

	_, err = server.applyAdminOps(ctx, session, door, []protocol.AdminOp{
		{Op: "set_door_enabled", DoorID: "chat", Enabled: boolPtr(false)},
	})
	if err != nil {
		t.Fatalf("applyAdminOps() error = %v", err)
	}

	events, err := store.ListAuditEvents(ctx, 0)
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Action != "admin.set_door_enabled" || events[0].Target != "chat" || events[0].ActorFingerprint == "" {
		t.Fatalf("audit event = %#v, want door enabled audit event", events[0])
	}
	if events[0].Metadata["enabled"] != false || events[0].Metadata["source_door"] != adminDoorID {
		t.Fatalf("audit metadata = %#v, want enabled=false and source door", events[0].Metadata)
	}

	lines := strings.Split(strings.TrimSpace(auditFile.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("audit file lines = %d, want 1; content=%q", len(lines), auditFile.String())
	}
	var fileEvent storage.AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &fileEvent); err != nil {
		t.Fatalf("decode audit file JSONL: %v", err)
	}
	if fileEvent.ID != events[0].ID || fileEvent.Action != events[0].Action {
		t.Fatalf("file audit event = %#v, want mirrored db event %#v", fileEvent, events[0])
	}
}

func TestServerAuditMaxBytesTrimsSQLiteEvents(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	server := newServerWithOptions(config.DefaultNodeConfig(), nil, store, serverOptions{AuditLogMaxBytes: 120})
	server.audit(ctx, auditEvent("ed25519:one", "auth.denied", "station", "denied", map[string]any{"reason": strings.Repeat("one", 20)}))
	server.audit(ctx, auditEvent("ed25519:two", "auth.denied", "station", "denied", map[string]any{"reason": strings.Repeat("two", 20)}))

	events, err := store.ListAuditEvents(ctx, 0)
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want one event after max-byte trim", len(events))
	}
	if events[0].ActorPublicKey != "ed25519:two" {
		t.Fatalf("retained actor = %q, want newest event actor", events[0].ActorPublicKey)
	}
}

func TestRememberNodeIdentityAuditsChangedNodeKey(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveNodeState(ctx, nodeIdentityStateKey, map[string]any{"public_key": "ed25519:old"}); err != nil {
		t.Fatalf("SaveNodeState() error = %v", err)
	}
	cfg := config.DefaultNodeConfig()
	cfg.Name = "localbox"
	cfg.NodeID = "ed25519:new"
	server := newServer(cfg, nil, store)

	if err := server.rememberNodeIdentity(ctx); err != nil {
		t.Fatalf("rememberNodeIdentity() error = %v", err)
	}
	events, err := store.ListAuditEvents(ctx, 0)
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Action != "node.key_changed" || events[0].Target != "localbox" {
		t.Fatalf("audit event = %#v, want node key change", events[0])
	}
	if events[0].Metadata["previous_public_key"] != "ed25519:old" || events[0].Metadata["current_public_key"] != "ed25519:new" {
		t.Fatalf("metadata = %#v, want old and new public keys", events[0].Metadata)
	}
}

func TestRotatingAuditFileKeepsLimitedBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	file, err := openRotatingAuditFile(path, 24, 2)
	if err != nil {
		t.Fatalf("openRotatingAuditFile() error = %v", err)
	}
	defer file.Close()

	for i := 0; i < 4; i++ {
		if _, err := file.Write([]byte("0123456789abcdef\n")); err != nil {
			t.Fatalf("Write(%d) error = %v", i, err)
		}
	}

	for _, name := range []string{path, path + ".1", path + ".2"} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty after rotation", name)
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("stat %s error = %v, want no third backup", path+".3", err)
	}
}
