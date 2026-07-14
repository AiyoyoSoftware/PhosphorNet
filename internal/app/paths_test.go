package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandUserPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := ExpandUserPath("~/.local/share/phosphornet/actiond.sock")
	if err != nil {
		t.Fatalf("ExpandUserPath() error = %v", err)
	}
	want := filepath.Join(home, ".local", "share", "phosphornet", "actiond.sock")
	if got != want {
		t.Fatalf("ExpandUserPath() = %q, want %q", got, want)
	}
}

func TestHomeActiondSocketPathUsesUserDataDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".local", "share", "phosphornet", "actiond.sock")
	if got := HomeActiondSocketPath(); got != want {
		t.Fatalf("HomeActiondSocketPath() = %q, want %q", got, want)
	}
}

func TestInstalledDefaultsUseBoringSystemLocations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := DefaultNodeConfigPath(); got != SystemNodeConfigPath {
		t.Fatalf("DefaultNodeConfigPath() = %q, want %q", got, SystemNodeConfigPath)
	}
	if got := DefaultDoorsDir(); got != SystemDoorsDir {
		t.Fatalf("DefaultDoorsDir() = %q, want %q", got, SystemDoorsDir)
	}
	if got := DefaultDatabasePath(); got != SystemDatabasePath {
		t.Fatalf("DefaultDatabasePath() = %q, want %q", got, SystemDatabasePath)
	}
}

func TestHomeFilesOverrideInstalledDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	homeConfig := filepath.Join(home, ".config", "phosphornet", "node.toml")
	homeDoors := filepath.Join(home, ".local", "share", "phosphornet", "doors")
	homeDatabase := filepath.Join(home, ".local", "share", "phosphornet", "phosphornet.db")
	if err := os.MkdirAll(filepath.Dir(homeConfig), 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	if err := os.WriteFile(homeConfig, []byte("name = \"home\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := os.MkdirAll(homeDoors, 0o755); err != nil {
		t.Fatalf("MkdirAll(doors) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(homeDatabase), 0o755); err != nil {
		t.Fatalf("MkdirAll(database parent) error = %v", err)
	}
	if err := os.WriteFile(homeDatabase, []byte{}, 0o600); err != nil {
		t.Fatalf("WriteFile(database) error = %v", err)
	}

	if got := DefaultNodeConfigPath(); got != homeConfig {
		t.Fatalf("DefaultNodeConfigPath() = %q, want %q", got, homeConfig)
	}
	if got := DefaultDoorsDir(); got != homeDoors {
		t.Fatalf("DefaultDoorsDir() = %q, want %q", got, homeDoors)
	}
	if got := DefaultDatabasePath(); got != homeDatabase {
		t.Fatalf("DefaultDatabasePath() = %q, want %q", got, homeDatabase)
	}
}
