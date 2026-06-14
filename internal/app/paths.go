package app

import (
	"os"
	"path/filepath"
)

const (
	DefaultPassportName   = "passport.toml"
	DefaultKnownNodeName  = "known_nodes.toml"
	DefaultNodeConfigName = "node.toml"
	DefaultDatabaseName   = "phosphornet.db"

	SystemNodeConfigPath = "/etc/phosphornet/node.toml"
	SystemDoorsDir       = "/usr/local/share/phosphornet/doors"
	SystemDatabasePath   = "/var/lib/phosphornet/phosphornet.db"
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
