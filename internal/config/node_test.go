package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"phosphornet/internal/identity"
)

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
