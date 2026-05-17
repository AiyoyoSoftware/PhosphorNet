package runtime

import (
	"fmt"
	"strings"
)

const (
	IsolationModeHost   = "host"
	IsolationModePodman = "podman"

	IsolationNetworkNone   = "none"
	IsolationNetworkHost   = "host"
	IsolationNetworkBridge = "bridge"
)

type RuntimeOptions struct {
	DefaultRuntime string           `toml:"default_runtime"`
	Lua            LuaSandboxConfig `toml:"lua"`
}

type LuaSandboxConfig struct {
	Profile          string   `toml:"profile"`
	Libraries        []string `toml:"libraries"`
	MaxMemoryKB      int      `toml:"max_memory_kb"`
	MaxExecutionMS   int      `toml:"max_execution_ms"`
	CallStackSize    int      `toml:"call_stack_size"`
	RegistrySize     int      `toml:"registry_size"`
	RegistryMaxSize  int      `toml:"registry_max_size"`
	RegistryGrowStep int      `toml:"registry_grow_step"`
}

func DefaultRuntimeOptions() RuntimeOptions {
	return RuntimeOptions{
		DefaultRuntime: "lua",
		Lua: LuaSandboxConfig{
			Profile:          "strict",
			MaxMemoryKB:      64 * 1024,
			MaxExecutionMS:   5000,
			CallStackSize:    120,
			RegistrySize:     20 * 1024,
			RegistryMaxSize:  80 * 1024,
			RegistryGrowStep: 32,
		},
	}
}

func (o RuntimeOptions) withDefaults() RuntimeOptions {
	defaults := DefaultRuntimeOptions()
	if o.DefaultRuntime == "" {
		o.DefaultRuntime = defaults.DefaultRuntime
	}
	o.Lua = defaults.Lua.merge(o.Lua)
	return o
}

func (c LuaSandboxConfig) merge(override LuaSandboxConfig) LuaSandboxConfig {
	if override.Profile != "" {
		c.Profile = override.Profile
	}
	if len(override.Libraries) > 0 {
		c.Libraries = override.Libraries
	}
	if override.MaxMemoryKB != 0 {
		c.MaxMemoryKB = override.MaxMemoryKB
	}
	if override.MaxExecutionMS != 0 {
		c.MaxExecutionMS = override.MaxExecutionMS
	}
	if override.CallStackSize != 0 {
		c.CallStackSize = override.CallStackSize
	}
	if override.RegistrySize != 0 {
		c.RegistrySize = override.RegistrySize
	}
	if override.RegistryMaxSize != 0 {
		c.RegistryMaxSize = override.RegistryMaxSize
	}
	if override.RegistryGrowStep != 0 {
		c.RegistryGrowStep = override.RegistryGrowStep
	}
	return c
}

func normalizeSandboxProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "strict":
		return "strict"
	case "standard":
		return "standard"
	case "unsafe":
		return "strict"
	default:
		return "custom"
	}
}

func NormalizeSandboxProfileForDisplay(profile string) string {
	return normalizeSandboxProfile(profile)
}

func ValidateRuntimeName(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "lua", "stdio":
		return nil
	default:
		return fmt.Errorf("unknown runtime %q", name)
	}
}

func ValidateLuaSandboxConfig(cfg LuaSandboxConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Profile)) {
	case "", "strict", "standard", "unsafe":
		return nil
	case "custom":
		if len(cfg.Libraries) == 0 {
			return fmt.Errorf("custom lua sandbox profile requires explicit libraries")
		}
		return nil
	default:
		return fmt.Errorf("unknown lua sandbox profile %q", cfg.Profile)
	}
}

func ValidateDoorIsolationConfig(runtimeName string, cfg DoorIsolationConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if stdioRuntimeUsesIsolation(runtimeName) {
			mode = IsolationModePodman
		} else if hasDoorIsolationFields(cfg) {
			return fmt.Errorf("mode is required when isolation is configured")
		} else {
			return nil
		}
	}
	if !stdioRuntimeUsesIsolation(runtimeName) {
		return fmt.Errorf("isolation mode %q is only supported for stdio runtime", cfg.Mode)
	}
	switch mode {
	case IsolationModeHost:
		if strings.TrimSpace(cfg.Image) != "" {
			return fmt.Errorf("host isolation mode must not set image")
		}
		if strings.TrimSpace(cfg.Network) != "" {
			return fmt.Errorf("host isolation mode must not set network")
		}
		if cfg.ReadOnly != nil {
			return fmt.Errorf("host isolation mode must not set read_only")
		}
		if strings.TrimSpace(cfg.Memory) != "" {
			return fmt.Errorf("host isolation mode must not set memory")
		}
		if cfg.CPUs != 0 {
			return fmt.Errorf("host isolation mode must not set cpus")
		}
		if cfg.PidsLimit != 0 {
			return fmt.Errorf("host isolation mode must not set pids_limit")
		}
	case IsolationModePodman:
		if strings.TrimSpace(cfg.Image) == "" {
			return fmt.Errorf("podman isolation requires image")
		}
	default:
		return fmt.Errorf("unknown isolation mode %q", cfg.Mode)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Network)) {
	case "", IsolationNetworkNone, IsolationNetworkHost, IsolationNetworkBridge:
	default:
		return fmt.Errorf("unknown network %q", cfg.Network)
	}
	if cfg.TimeoutMS < 0 {
		return fmt.Errorf("timeout_ms must be non-negative")
	}
	if cfg.CPUs < 0 {
		return fmt.Errorf("cpus must be non-negative")
	}
	if cfg.PidsLimit < 0 {
		return fmt.Errorf("pids_limit must be non-negative")
	}
	return nil
}

func stdioRuntimeUsesIsolation(runtimeName string) bool {
	switch strings.ToLower(strings.TrimSpace(runtimeName)) {
	case "stdio":
		return true
	default:
		return false
	}
}

func hasDoorIsolationFields(cfg DoorIsolationConfig) bool {
	return strings.TrimSpace(cfg.Image) != "" ||
		strings.TrimSpace(cfg.Network) != "" ||
		cfg.ReadOnly != nil ||
		cfg.TimeoutMS != 0 ||
		strings.TrimSpace(cfg.Memory) != "" ||
		cfg.CPUs != 0 ||
		cfg.PidsLimit != 0
}
