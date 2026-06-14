package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"phosphornet/internal/app"
	"phosphornet/internal/identity"
)

func TestDefaultNodeConfigUsesInstalledPathsWithoutHomeOverrides(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultNodeConfig()
	if cfg.DoorsDir != app.SystemDoorsDir {
		t.Fatalf("DoorsDir = %q, want %q", cfg.DoorsDir, app.SystemDoorsDir)
	}
	if cfg.Database != app.SystemDatabasePath {
		t.Fatalf("Database = %q, want %q", cfg.Database, app.SystemDatabasePath)
	}
}

func TestApplyHomeOverridesUsesHomeDataWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	homeDoors := filepath.Join(home, ".local", "share", "phosphornet", "doors")
	homeDatabase := filepath.Join(home, ".local", "share", "phosphornet", "phosphornet.db")
	if err := os.MkdirAll(homeDoors, 0o755); err != nil {
		t.Fatalf("MkdirAll(doors) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(homeDatabase), 0o755); err != nil {
		t.Fatalf("MkdirAll(database parent) error = %v", err)
	}
	if err := os.WriteFile(homeDatabase, []byte{}, 0o600); err != nil {
		t.Fatalf("WriteFile(database) error = %v", err)
	}

	cfg := ApplyHomeOverrides(DefaultSystemNodeConfig())
	if cfg.DoorsDir != homeDoors {
		t.Fatalf("DoorsDir = %q, want %q", cfg.DoorsDir, homeDoors)
	}
	if cfg.Database != homeDatabase {
		t.Fatalf("Database = %q, want %q", cfg.Database, homeDatabase)
	}
}

func TestLoadNodeConfigRejectsUnknownFields(t *testing.T) {
	passport, err := identity.Generate("node")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "node.toml")
	content := fmt.Sprintf("name = \"localbox\"\nlisten_addr = \":7707\"\nnode_id = %q\nprivate_key = %q\ndoors_dir = \"./doors\"\ndatabase = \"./phosphornet.db\"\nunknown_field = true\n", passport.PublicKey, passport.PrivateKey)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadNodeConfig(path); err == nil {
		t.Fatal("LoadNodeConfig() error = nil, want unknown field rejection")
	}
}

func TestLoadNodeConfigRejectsInvalidAccessMode(t *testing.T) {
	passport, err := identity.Generate("node")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "node.toml")
	content := fmt.Sprintf("name = \"localbox\"\nlisten_addr = \":7707\"\nnode_id = %q\nprivate_key = %q\ndoors_dir = \"./doors\"\ndatabase = \"./phosphornet.db\"\n[access]\nmode = \"publci\"\n", passport.PublicKey, passport.PrivateKey)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadNodeConfig(path); err == nil {
		t.Fatal("LoadNodeConfig() error = nil, want invalid access.mode rejection")
	}
}
