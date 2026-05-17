package app

import (
	"os"
	"path/filepath"
)

const (
	DefaultPassportName  = "passport.toml"
	DefaultKnownNodeName = "known_nodes.toml"
)

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "phosphornet")
}

func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0o755)
}

func EnsureParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}

func DefaultPassportPath() string {
	return filepath.Join(ConfigDir(), DefaultPassportName)
}

func DefaultKnownNodesPath() string {
	return filepath.Join(ConfigDir(), DefaultKnownNodeName)
}

func QuickTestDir() string {
	return filepath.Join(os.TempDir(), "phosphornet-quick")
}

func QuickTestPassportPath() string {
	return filepath.Join(QuickTestDir(), DefaultPassportName)
}

func QuickTestKnownNodesPath() string {
	return filepath.Join(QuickTestDir(), DefaultKnownNodeName)
}
