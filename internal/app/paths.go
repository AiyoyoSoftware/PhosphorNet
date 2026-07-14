package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ExpandUserPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

const (
	DefaultPassportName      = "passport.toml"
	DefaultKnownNodeName     = "known_nodes.toml"
	DefaultNodeConfigName    = "node.toml"
	DefaultActiondConfigName = "actiond.toml"
	DefaultDatabaseName      = "phosphornet.db"

	SystemNodeConfigPath    = "/etc/phosphornet/node.toml"
	SystemActiondConfigPath = "/etc/phosphornet/actiond.toml"
	SystemActiondSocketPath = "/run/phosphornet/actiond.sock"
	SystemDoorsDir          = "/usr/local/share/phosphornet/doors"
	SystemDatabasePath      = "/var/lib/phosphornet/phosphornet.db"
)

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "phosphornet")
}

func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "share", "phosphornet")
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

func HomeNodeConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultNodeConfigName)
}

func HomeActiondConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultActiondConfigName)
}

func HomeActiondSocketPath() string {
	return filepath.Join(DataDir(), "actiond.sock")
}

func HomeDoorsDir() string {
	return filepath.Join(DataDir(), "doors")
}

func HomeDatabasePath() string {
	return filepath.Join(DataDir(), DefaultDatabaseName)
}

func DefaultNodeConfigPath() string {
	if pathExists(HomeNodeConfigPath()) {
		return HomeNodeConfigPath()
	}
	return SystemNodeConfigPath
}

func DefaultActiondConfigPath() string {
	if pathExists(HomeActiondConfigPath()) {
		return HomeActiondConfigPath()
	}
	return SystemActiondConfigPath
}

func DefaultDoorsDir() string {
	if pathExists(HomeDoorsDir()) {
		return HomeDoorsDir()
	}
	return SystemDoorsDir
}

func DefaultDatabasePath() string {
	if pathExists(HomeDatabasePath()) {
		return HomeDatabasePath()
	}
	return SystemDatabasePath
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

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
