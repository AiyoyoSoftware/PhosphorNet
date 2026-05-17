package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"phosphornet/internal/protocol"
)

func TestScopedStateOpsAreAtomicAcrossScopes(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	ids := StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}
	err = store.ApplyStateOps(ctx, "chat", ids, "member", []protocol.StateOp{
		{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "draft", Value: "hello"},
		{Scope: protocol.StateScopeRoom, Op: protocol.StateOpSet, Key: "topic", Value: "general"},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}

	state, err := store.LoadScopedState(ctx, "chat", ids)
	if err != nil {
		t.Fatalf("LoadScopedState() error = %v", err)
	}
	if state.User["draft"] != "hello" {
		t.Fatalf("state.User[draft] = %v, want hello", state.User["draft"])
	}
	if state.Room["topic"] != "general" {
		t.Fatalf("state.Room[topic] = %v, want general", state.Room["topic"])
	}
}

func TestOpenSQLiteReportsCurrentSchemaVersion(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion() error = %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("SchemaVersion() = %d, want %d", version, CurrentSchemaVersion)
	}
	if store.Path() == "" || !filepath.IsAbs(store.Path()) {
		t.Fatalf("Path() = %q, want absolute path", store.Path())
	}
	if !store.Created() {
		t.Fatal("Created() = false, want true for a new database")
	}
	path := store.Path()
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite(reopen) error = %v", err)
	}
	defer reopened.Close()
	if reopened.Created() {
		t.Fatal("Created() = true, want false for an existing database")
	}
}

func TestGlobalStateWriteRequiresAdminRole(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	ids := StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}
	err = store.ApplyStateOps(ctx, "chat", ids, "member", []protocol.StateOp{
		{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "draft", Value: "hello"},
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "motd", Value: "admin only"},
	})
	if err == nil {
		t.Fatal("ApplyStateOps() error = nil, want permission error")
	}

	state, err := store.LoadScopedState(ctx, "chat", ids)
	if err != nil {
		t.Fatalf("LoadScopedState() error = %v", err)
	}
	if _, ok := state.User["draft"]; ok {
		t.Fatal("user state changed even though global write rejected")
	}
	if _, ok := state.Global["motd"]; ok {
		t.Fatal("global state changed for non-admin user")
	}
}

func TestUsersTrackLastSeen(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	if err := store.RecordUser(ctx, "ed25519:user"); err != nil {
		t.Fatalf("RecordUser() error = %v", err)
	}
	if err := store.RecordUser(ctx, "ed25519:user"); err != nil {
		t.Fatalf("RecordUser() second error = %v", err)
	}
	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("len(users) = %d, want 1", len(users))
	}
	if users[0].FirstSeen == "" || users[0].LastSeen == "" {
		t.Fatalf("user timestamps = %#v, want first_seen and last_seen", users[0])
	}
}

func TestListStateRecords(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	ids := StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}
	err = store.ApplyStateOps(ctx, "chat", ids, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "draft", Value: "hello"},
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "motd", Value: "welcome"},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}
	records, err := store.ListStateRecords(ctx)
	if err != nil {
		t.Fatalf("ListStateRecords() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	for _, record := range records {
		if record.DoorID != "chat" || record.Bytes == 0 || record.UpdatedAt == "" {
			t.Fatalf("record = %#v, want populated chat state summary", record)
		}
	}
}

func TestSaveUserProfileStoresDisplayNameAndProfileFields(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	profile, err := store.SaveUserProfile(ctx, "ed25519:user", protocol.ProfileUpdateEffect{
		DisplayName: stringPtr("ada"),
		Bio:         stringPtr("likes terminal stations"),
		StatusLine:  stringPtr("in the lobby"),
	})
	if err != nil {
		t.Fatalf("SaveUserProfile() error = %v", err)
	}
	if profile.DisplayName != "ada" || profile.Bio != "likes terminal stations" || profile.StatusLine != "in the lobby" {
		t.Fatalf("profile = %#v, want saved display-name fields", profile)
	}

	loaded, err := store.LoadUserProfile(ctx, "ed25519:user")
	if err != nil {
		t.Fatalf("LoadUserProfile() error = %v", err)
	}
	if loaded.DisplayName != "ada" || loaded.Bio != "likes terminal stations" || loaded.StatusLine != "in the lobby" {
		t.Fatalf("loaded = %#v, want saved display-name fields", loaded)
	}
}

func TestSaveUserProfileRejectsReservedDisplayName(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	_, err = store.SaveUserProfile(ctx, "ed25519:user", protocol.ProfileUpdateEffect{
		DisplayName: stringPtr("admin"),
	})
	if err == nil {
		t.Fatal("SaveUserProfile() error = nil, want reserved-name rejection")
	}
}

func TestApplyStateOpsRejectsInvalidStateKey(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	err = store.ApplyStateOps(ctx, "chat", StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}, "member", []protocol.StateOp{
		{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "../../otherdoor", Value: "nope"},
	})
	if err == nil {
		t.Fatal("ApplyStateOps() error = nil, want invalid key rejection")
	}
}

func TestApplyStateOpsRejectsOversizedValue(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	err = store.ApplyStateOps(ctx, "chat", StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}, "member", []protocol.StateOp{
		{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "draft", Value: strings.Repeat("x", protocol.MaxStateValueJSONBytes+1)},
	})
	if err == nil {
		t.Fatal("ApplyStateOps() error = nil, want oversized value rejection")
	}
}

func TestApplyStateOpsRejectsOversizedBatch(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	ops := make([]protocol.StateOp, 0, protocol.MaxStateOpsPerBatch+1)
	for i := 0; i < protocol.MaxStateOpsPerBatch+1; i++ {
		ops = append(ops, protocol.StateOp{Scope: protocol.StateScopeUser, Op: protocol.StateOpSet, Key: "draft", Value: i})
	}
	err = store.ApplyStateOps(ctx, "chat", StateScopeIDs{User: "user-a", Room: "door:chat", Global: "global"}, "member", ops)
	if err == nil {
		t.Fatal("ApplyStateOps() error = nil, want oversized batch rejection")
	}
}

func stringPtr(value string) *string {
	return &value
}
