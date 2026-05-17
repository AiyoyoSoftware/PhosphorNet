package node

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/identity"
	"phosphornet/internal/storage"
)

func TestEnsureAdminPassportCreatesAndReusesPassport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passport.toml")

	createdPassport, created, err := ensureAdminPassport(path)
	if err != nil {
		t.Fatalf("ensureAdminPassport() error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true for missing passport")
	}

	reusedPassport, created, err := ensureAdminPassport(path)
	if err != nil {
		t.Fatalf("ensureAdminPassport() reuse error = %v", err)
	}
	if created {
		t.Fatal("created = true, want false for existing passport")
	}
	if reusedPassport.PublicKey != createdPassport.PublicKey {
		t.Fatalf("reused public key = %q, want %q", reusedPassport.PublicKey, createdPassport.PublicKey)
	}
}

func TestInitCommandSeedsAdminAccessFromPassport(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "node.toml")
	passportPath := filepath.Join(dir, "passport.toml")

	cmd := newInitCommand()
	cmd.SetArgs([]string{
		"--name", "localbox",
		"--out", configPath,
		"--admin-passport", passportPath,
	})
	cmd.SetOut(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command error = %v", err)
	}

	cfg, err := config.LoadNodeConfig(configPath)
	if err != nil {
		t.Fatalf("LoadNodeConfig() error = %v", err)
	}
	admin, err := identity.Load(passportPath)
	if err != nil {
		t.Fatalf("Load admin passport error = %v", err)
	}
	if len(cfg.Access.Admins) != 1 || cfg.Access.Admins[0] != admin.PublicKey {
		t.Fatalf("cfg.Access.Admins = %#v, want admin public key", cfg.Access.Admins)
	}

	store, err := storage.OpenSQLite(cfg.Database)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	state, err := store.LoadScopedState(context.Background(), adminDoorID, storage.StateScopeIDs{Global: "global"})
	if err != nil {
		t.Fatalf("LoadScopedState() error = %v", err)
	}
	disabled := boolMapFromAnyMap(state.Global["disabled_doors"])
	if !disabled["strategy_demo"] {
		t.Fatalf("disabled_doors = %#v, want strategy_demo disabled by default", disabled)
	}
}
