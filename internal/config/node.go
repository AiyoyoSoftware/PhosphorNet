package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"phosphornet/internal/identity"
	"phosphornet/internal/runtime"
)

type NodeConfig struct {
	Name       string                 `toml:"name"`
	ListenAddr string                 `toml:"listen_addr"`
	NodeID     string                 `toml:"node_id"`
	PrivateKey string                 `toml:"private_key"`
	DoorsDir   string                 `toml:"doors_dir"`
	Database   string                 `toml:"database"`
	TLS        TLSConfig              `toml:"tls"`
	Access     StationAccessConfig    `toml:"access"`
	Runtime    runtime.RuntimeOptions `toml:"runtime"`
}

type TLSConfig struct {
	Enabled bool `toml:"enabled"`
}

type StationAccessConfig struct {
	Mode      string   `toml:"mode"`
	Allowlist []string `toml:"allowlist"`
	Admins    []string `toml:"admins"`
}

func DefaultNodeConfig() NodeConfig {
	return NodeConfig{
		Name:       "localbox",
		ListenAddr: ":7707",
		DoorsDir:   "./doors",
		Database:   "./phosphornet.db",
		TLS: TLSConfig{
			Enabled: true,
		},
		Access: StationAccessConfig{
			Mode: "public",
		},
		Runtime: runtime.DefaultRuntimeOptions(),
	}
}

func LoadNodeConfig(path string) (NodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NodeConfig{}, fmt.Errorf("read node config: %w", err)
	}
	cfg := DefaultNodeConfig()
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return NodeConfig{}, fmt.Errorf("parse node config: %w", err)
	}
	if err := validateNodeConfig(cfg); err != nil {
		return NodeConfig{}, fmt.Errorf("validate node config: %w", err)
	}
	return cfg, nil
}

func SaveNodeConfig(path string, cfg NodeConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal node config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func validateNodeConfig(cfg NodeConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return fmt.Errorf("listen_addr is required")
	}
	if strings.TrimSpace(cfg.DoorsDir) == "" {
		return fmt.Errorf("doors_dir is required")
	}
	if strings.TrimSpace(cfg.Database) == "" {
		return fmt.Errorf("database is required")
	}
	if err := validateStationAccessMode(cfg.Access.Mode); err != nil {
		return err
	}
	if err := runtime.ValidateRuntimeName(cfg.Runtime.DefaultRuntime); err != nil {
		return fmt.Errorf("runtime.default_runtime: %w", err)
	}
	if err := runtime.ValidateLuaSandboxConfig(cfg.Runtime.Lua); err != nil {
		return fmt.Errorf("runtime.lua: %w", err)
	}
	passport := &identity.Passport{
		DisplayName: cfg.Name,
		PublicKey:   cfg.NodeID,
		PrivateKey:  cfg.PrivateKey,
	}
	if err := passport.Validate(); err != nil {
		return fmt.Errorf("node identity: %w", err)
	}
	return nil
}

func validateStationAccessMode(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "public", "invite_only":
		return nil
	default:
		return fmt.Errorf("unknown access.mode %q", value)
	}
}
