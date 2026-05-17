package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func TestInviteOnlyStationAllowlist(t *testing.T) {
	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	server := newServer(config.NodeConfig{
		Access: config.StationAccessConfig{
			Mode:      accessInviteOnly,
			Allowlist: []string{passport.Fingerprint()},
		},
	}, nil, nil)

	ctx := context.Background()
	if !server.stationAllows(ctx, passport.PublicKey) {
		t.Fatal("stationAllows() = false, want true for allowlisted fingerprint")
	}
	if server.stationAllows(ctx, "not-invited") {
		t.Fatal("stationAllows() = true, want false for non-allowlisted key")
	}
}

func TestAdminsAreAllowedAndReceiveAdminRole(t *testing.T) {
	passport, err := identity.Generate("admin")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	server := newServer(config.NodeConfig{
		Access: config.StationAccessConfig{
			Mode:   accessInviteOnly,
			Admins: []string{passport.PublicKey},
		},
	}, nil, nil)

	ctx := context.Background()
	if !server.stationAllows(ctx, passport.PublicKey) {
		t.Fatal("stationAllows() = false, want true for admin on invite-only station")
	}
	if got := server.roleForPublicKey(ctx, passport.PublicKey); got != "admin" {
		t.Fatalf("roleForPublicKey() = %q, want admin", got)
	}
}

func TestDoorInviteOnlyAccess(t *testing.T) {
	passport, err := identity.Generate("traveler")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	server := newServer(config.DefaultNodeConfig(), nil, nil)
	session := &sessionState{publicKey: passport.PublicKey, role: "member"}
	door := runtime.DoorManifest{
		ID:        "members",
		Access:    accessInviteOnly,
		Allowlist: []string{passport.PublicKey},
	}

	ctx := context.Background()
	if !server.canAccessDoor(ctx, session, door) {
		t.Fatal("canAccessDoor() = false, want true for allowlisted key")
	}

	door.Allowlist = nil
	if server.canAccessDoor(ctx, session, door) {
		t.Fatal("canAccessDoor() = true, want false for invite-only door without allowlist match")
	}
}

func TestAdminDoorAccess(t *testing.T) {
	server := newServer(config.DefaultNodeConfig(), nil, nil)
	door := runtime.DoorManifest{
		ID:     "admin",
		Access: accessAdmin,
	}

	ctx := context.Background()
	if server.canAccessDoor(ctx, &sessionState{publicKey: "member", role: "member"}, door) {
		t.Fatal("canAccessDoor() = true, want false for member admin door access")
	}
	if !server.canAccessDoor(ctx, &sessionState{publicKey: "admin", role: "admin"}, door) {
		t.Fatal("canAccessDoor() = false, want true for admin admin door access")
	}
}

func TestStationPolicyRolesDoNotBypassInviteOnlyAdmission(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	passport, err := identity.Generate("moderator")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	err = store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "roles", Value: map[string]any{passport.PublicKey: "moderator"}},
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "door_roles", Value: map[string]any{"ops": []any{"moderator"}}},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}
	server := newServer(config.NodeConfig{
		Access: config.StationAccessConfig{Mode: accessInviteOnly},
	}, nil, store)

	if server.stationAllows(ctx, passport.PublicKey) {
		t.Fatal("stationAllows() = true, want role assignment not to bypass invite-only station admission")
	}
	if got := server.roleForPublicKey(ctx, passport.PublicKey); got != "moderator" {
		t.Fatalf("roleForPublicKey() = %q, want moderator", got)
	}
}

func TestStationPolicyRoleDoorAccessStillUsesAssignedRole(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	passport, err := identity.Generate("moderator")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	err = store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "roles", Value: map[string]any{passport.PublicKey: "moderator"}},
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "door_roles", Value: map[string]any{"ops": []any{"moderator"}}},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}
	server := newServer(config.DefaultNodeConfig(), nil, store)

	if !server.canAccessDoor(ctx, &sessionState{publicKey: passport.PublicKey, role: "moderator"}, runtime.DoorManifest{ID: "ops"}) {
		t.Fatal("canAccessDoor() = false, want moderator role to access role-gated door")
	}
	if server.canAccessDoor(ctx, &sessionState{publicKey: "member", role: "member"}, runtime.DoorManifest{ID: "ops"}) {
		t.Fatal("canAccessDoor() = true, want member denied from role-gated door")
	}
}

func TestStationPolicyDisabledDoorsDenyMembers(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	err = store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "disabled_doors", Value: map[string]any{"chat": true}},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}
	door := runtime.DoorManifest{ID: "chat"}
	server := newServer(config.DefaultNodeConfig(), []runtime.DoorManifest{door}, store)

	if server.canAccessDoor(ctx, &sessionState{publicKey: "member", role: "member"}, door) {
		t.Fatal("canAccessDoor() = true, want disabled door denied for member")
	}
	if server.canAccessDoor(ctx, &sessionState{publicKey: "admin", role: "admin"}, door) {
		t.Fatal("canAccessDoor() = true, want disabled door denied for admin outside admin panel")
	}
	summary := doorSummaryWithPolicy(doorSummary(door), server.loadStationPolicy(ctx))
	if !summary.Disabled {
		t.Fatalf("summary.Disabled = false, want true")
	}
	memberDoors := server.visibleDoorSummaries(ctx, &sessionState{publicKey: "member", role: "member"})
	if len(memberDoors) != 0 {
		t.Fatalf("member visible doors = %#v, want disabled door hidden", memberDoors)
	}
	adminDoors := server.visibleDoorSummaries(ctx, &sessionState{publicKey: "admin", role: "admin"})
	if len(adminDoors) != 0 {
		t.Fatalf("admin visible doors = %#v, want disabled door hidden outside admin panel", adminDoors)
	}
	adminInventory := server.allDoorSummaries(ctx)
	if len(adminInventory) != 1 || !adminInventory[0].Disabled {
		t.Fatalf("admin inventory = %#v, want disabled metadata visible in admin panel", adminInventory)
	}
}

func TestStationPolicyBannedKeyDeniesStationAndDoorAccess(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	server := newServer(config.DefaultNodeConfig(), []runtime.DoorManifest{{ID: "lobby"}}, store)
	policy := server.loadStationPolicy(ctx)
	policy.Moderation.BannedKeys["ed25519:bad"] = moderationEntry{Reason: "spam", CreatedAt: "2026-05-14T00:00:00Z"}
	if err := server.saveStationPolicy(ctx, policy); err != nil {
		t.Fatalf("saveStationPolicy() error = %v", err)
	}

	if server.stationAllows(ctx, "ed25519:bad") {
		t.Fatal("stationAllows() = true, want banned key denied")
	}
	if server.canAccessDoor(ctx, &sessionState{publicKey: "ed25519:bad", role: "member"}, runtime.DoorManifest{ID: "lobby"}) {
		t.Fatal("canAccessDoor() = true, want banned key denied")
	}
}

func TestMutedMemberEventModerationAllowsNavigationOnly(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	server := newServer(config.DefaultNodeConfig(), nil, store)
	policy := server.loadStationPolicy(ctx)
	policy.Moderation.MutedKeys["ed25519:muted"] = moderationEntry{Reason: "flooding", CreatedAt: "2026-05-14T00:00:00Z"}
	if err := server.saveStationPolicy(ctx, policy); err != nil {
		t.Fatalf("saveStationPolicy() error = %v", err)
	}
	session := &sessionState{publicKey: "ed25519:muted", role: "member", activeDoor: "forum"}

	if err := server.enforceEventModeration(ctx, session, protocol.UIEvent{Kind: protocol.EventKindAction, Action: "nav:push:thread:1"}); err != nil {
		t.Fatalf("navigation action rejected for muted user: %v", err)
	}
	if err := server.enforceEventModeration(ctx, session, protocol.UIEvent{Kind: protocol.EventKindSubmit, Target: "forum-reply-body"}); err == nil {
		t.Fatal("submit event allowed for muted user, want rejection")
	}
	if err := server.enforceEventModeration(ctx, session, protocol.UIEvent{Kind: protocol.EventKindAction, Action: "post_reply"}); err == nil {
		t.Fatal("posting action allowed for muted user, want rejection")
	}
}

func TestPerUserEventRateLimit(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	server := newServer(config.DefaultNodeConfig(), nil, store)
	policy := server.loadStationPolicy(ctx)
	policy.Moderation.RateLimits["ed25519:noisy"] = userRateLimit{EventsPerMinute: 1}
	if err := server.saveStationPolicy(ctx, policy); err != nil {
		t.Fatalf("saveStationPolicy() error = %v", err)
	}
	session := &sessionState{publicKey: "ed25519:noisy", role: "member", activeDoor: "chat"}
	event := protocol.UIEvent{Kind: protocol.EventKindAction, Action: "ping"}

	if err := server.enforceEventModeration(ctx, session, event); err != nil {
		t.Fatalf("first event rejected: %v", err)
	}
	if err := server.enforceEventModeration(ctx, session, event); err == nil {
		t.Fatal("second event allowed, want rate-limit rejection")
	}
}

func TestRuntimeContextIncludesMutedPermission(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	doorDir := filepath.Join(dir, "mutecheck")
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(doorDir, "app.lua"), []byte(`
local ui = phosphornet.ui
function view(ctx)
  return ui.screen({ ui.text(tostring(ctx.permissions.muted)) })
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()
	cfg := config.DefaultNodeConfig()
	cfg.DoorsDir = dir
	server := newServer(cfg, nil, store)
	policy := server.loadStationPolicy(ctx)
	policy.Moderation.MutedKeys["ed25519:muted"] = moderationEntry{Reason: "flooding", CreatedAt: "2026-05-14T00:00:00Z"}
	if err := server.saveStationPolicy(ctx, policy); err != nil {
		t.Fatalf("saveStationPolicy() error = %v", err)
	}

	response, err := server.invokeDoorView(ctx, runtime.DoorManifest{
		ID:    "mutecheck",
		Name:  "Mute Check",
		Entry: "app.lua",
		Dir:   doorDir,
	}, &sessionState{id: "s1", publicKey: "ed25519:muted", role: "member"})
	if err != nil {
		t.Fatalf("invokeDoorView() error = %v", err)
	}
	if !uiTreeContainsText(response.View, "true") {
		t.Fatalf("view = %#v, want muted permission true", response.View)
	}
}

func TestStationPolicyDoorOrderSortsVisibleAndAdminDoorLists(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	err = store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{Scope: protocol.StateScopeGlobal, Op: protocol.StateOpSet, Key: "door_order", Value: []any{"forum", "lobby"}},
	})
	if err != nil {
		t.Fatalf("ApplyStateOps() error = %v", err)
	}
	server := newServer(config.DefaultNodeConfig(), []runtime.DoorManifest{
		{ID: "lobby", Name: "Lobby", Entry: "app.lua"},
		{ID: "chat", Name: "Chat", Entry: "app.lua"},
		{ID: "forum", Name: "Forum", Entry: "app.lua"},
	}, store)

	memberDoors := server.visibleDoorSummaries(ctx, &sessionState{publicKey: "member", role: "member"})
	if len(memberDoors) != 3 || memberDoors[0].ID != "forum" || memberDoors[1].ID != "lobby" || memberDoors[2].ID != "chat" {
		t.Fatalf("member visible doors = %#v, want forum/lobby/chat order", memberDoors)
	}

	adminDoors := server.allDoorSummaries(ctx)
	if len(adminDoors) != 3 || adminDoors[0].ID != "forum" || adminDoors[1].ID != "lobby" || adminDoors[2].ID != "chat" {
		t.Fatalf("admin doors = %#v, want forum/lobby/chat order", adminDoors)
	}
}

func TestDoorSummaryDefaultsAndMetadata(t *testing.T) {
	summary := doorSummary(runtime.DoorManifest{
		ID:         "quiet",
		Name:       "Quiet Room",
		Entry:      "app.py",
		Visibility: visibilityHidden,
		Access:     accessInviteOnly,
		Allowlist:  []string{"ed25519:one", "ed25519:two"},
	})

	if summary.Visibility != visibilityHidden {
		t.Fatalf("summary.Visibility = %q, want %q", summary.Visibility, visibilityHidden)
	}
	if summary.Access != accessInviteOnly {
		t.Fatalf("summary.Access = %q, want %q", summary.Access, accessInviteOnly)
	}
	if summary.Runtime != "python" {
		t.Fatalf("summary.Runtime = %q, want python", summary.Runtime)
	}
	if summary.Entry != "app.py" {
		t.Fatalf("summary.Entry = %q, want app.py", summary.Entry)
	}
	if summary.AllowlistCount != 2 {
		t.Fatalf("summary.AllowlistCount = %d, want 2", summary.AllowlistCount)
	}

	defaultSummary := doorSummary(runtime.DoorManifest{ID: "lobby", Name: "Lobby"})
	if defaultSummary.Visibility != visibilityPublic {
		t.Fatalf("default visibility = %q, want %q", defaultSummary.Visibility, visibilityPublic)
	}
	if defaultSummary.Access != accessPublic {
		t.Fatalf("default access = %q, want %q", defaultSummary.Access, accessPublic)
	}
	if defaultSummary.Runtime != "lua" {
		t.Fatalf("default runtime = %q, want lua", defaultSummary.Runtime)
	}
}

func TestDoorSummaryWithSettingsIncludesResolvedAdminValues(t *testing.T) {
	door := runtime.DoorManifest{
		ID:    "lobby",
		Name:  "Lobby",
		Entry: "app.lua",
		Settings: map[string]runtime.DoorSettingDefinition{
			"motd": {
				Type:    runtime.DoorSettingTypeTextarea,
				Label:   "Message of the day",
				Default: "Welcome",
			},
			"show_online_users": {
				Type:    runtime.DoorSettingTypeBool,
				Label:   "Show online users",
				Default: true,
			},
		},
	}

	summary := doorSummaryWithSettings(doorSummary(door), door, map[string]any{"motd": "Station night"})
	if len(summary.Settings) != 2 {
		t.Fatalf("len(summary.Settings) = %d, want 2", len(summary.Settings))
	}
	if summary.Settings[0].Name != "motd" || summary.Settings[0].Value != "Station night" {
		t.Fatalf("first setting = %#v, want resolved motd", summary.Settings[0])
	}
	if summary.Settings[1].Name != "show_online_users" || summary.Settings[1].Value != true {
		t.Fatalf("second setting = %#v, want default bool", summary.Settings[1])
	}
}

func TestReloadDoorManifestsLoadsNewDoors(t *testing.T) {
	ctx := context.Background()
	doorsDir := filepath.Join(t.TempDir(), "doors")
	if err := os.MkdirAll(filepath.Join(doorsDir, "lobby"), 0o755); err != nil {
		t.Fatalf("MkdirAll(lobby) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorsDir, "lobby", "app.lua"), []byte("function view(ctx) return { component = 'screen', children = {} } end"), 0o644); err != nil {
		t.Fatalf("WriteFile(lobby app) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorsDir, "lobby", "manifest.toml"), []byte("id = \"lobby\"\nname = \"Lobby\"\nentry = \"app.lua\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(lobby manifest) error = %v", err)
	}

	server := newServer(config.NodeConfig{DoorsDir: doorsDir}, []runtime.DoorManifest{
		{ID: "lobby", Name: "Lobby", Entry: "app.lua"},
	}, nil)

	if _, ok := server.findDoor("profile"); ok {
		t.Fatal("findDoor(profile) = true before reload, want false")
	}

	if err := os.MkdirAll(filepath.Join(doorsDir, "profile"), 0o755); err != nil {
		t.Fatalf("MkdirAll(profile) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorsDir, "profile", "app.lua"), []byte("function view(ctx) return { component = 'screen', children = {} } end"), 0o644); err != nil {
		t.Fatalf("WriteFile(profile app) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorsDir, "profile", "manifest.toml"), []byte("id = \"profile\"\nname = \"Profile\"\nentry = \"app.lua\"\nruntime = \"lua\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(profile manifest) error = %v", err)
	}

	if err := server.reloadDoorManifests(ctx); err != nil {
		t.Fatalf("reloadDoorManifests() error = %v", err)
	}
	if _, ok := server.findDoor("profile"); !ok {
		t.Fatal("findDoor(profile) = false after reload, want true")
	}
}
