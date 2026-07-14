package action

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"phosphornet/internal/app"
)

const (
	DefaultSocketPath      = app.SystemActiondSocketPath
	DefaultMaxRequestBytes = 64 * 1024
	DefaultMaxOutputBytes  = 64 * 1024
	MaxRequestBytes        = 1024 * 1024
	MaxOutputBytes         = 1024 * 1024
	MaxResponseBytes       = 16 * 1024 * 1024
	DefaultTimeoutMS       = 5000
	MaxTimeoutMS           = 60_000
)

var RuleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	Socket          string `toml:"socket"`
	MaxRequestBytes int64  `toml:"max_request_bytes"`
	MaxOutputBytes  int    `toml:"max_output_bytes"`
	Rules           []Rule `toml:"rules"`
}

type Rule struct {
	ID           string            `toml:"id"`
	AllowedDoors []string          `toml:"allowed_doors"`
	Command      []string          `toml:"command"`
	TimeoutMS    int               `toml:"timeout_ms"`
	WorkingDir   string            `toml:"working_dir"`
	Environment  map[string]string `toml:"environment"`
}

func DefaultConfig() Config {
	return Config{
		Socket:          DefaultSocketPath,
		MaxRequestBytes: DefaultMaxRequestBytes,
		MaxOutputBytes:  DefaultMaxOutputBytes,
	}
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read actiond config: %w", err)
	}
	cfg := DefaultConfig()
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse actiond config: %w", err)
	}
	cfg.Socket, err = app.ExpandUserPath(cfg.Socket)
	if err != nil {
		return Config{}, fmt.Errorf("expand actiond socket path: %w", err)
	}
	if err := ValidateConfig(cfg); err != nil {
		return Config{}, fmt.Errorf("validate actiond config: %w", err)
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validate actiond config: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal actiond config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func ValidateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Socket) == "" {
		return fmt.Errorf("socket is required")
	}
	if !filepath.IsAbs(cfg.Socket) {
		return fmt.Errorf("socket must be an absolute path")
	}
	if cfg.MaxRequestBytes <= 0 || cfg.MaxRequestBytes > MaxRequestBytes {
		return fmt.Errorf("max_request_bytes must be between 1 and %d", MaxRequestBytes)
	}
	if cfg.MaxOutputBytes <= 0 || cfg.MaxOutputBytes > MaxOutputBytes {
		return fmt.Errorf("max_output_bytes must be between 1 and %d", MaxOutputBytes)
	}
	seen := map[string]bool{}
	for index := range cfg.Rules {
		rule := cfg.Rules[index]
		if !RuleIDPattern.MatchString(rule.ID) {
			return fmt.Errorf("rules[%d].id %q is invalid", index, rule.ID)
		}
		if seen[rule.ID] {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seen[rule.ID] = true
		if len(rule.AllowedDoors) == 0 {
			return fmt.Errorf("rule %q requires at least one allowed_doors entry", rule.ID)
		}
		for _, doorID := range rule.AllowedDoors {
			if strings.TrimSpace(doorID) == "" || doorID == "*" {
				return fmt.Errorf("rule %q allowed_doors must contain explicit door ids", rule.ID)
			}
		}
		if len(rule.Command) == 0 || !filepath.IsAbs(rule.Command[0]) {
			return fmt.Errorf("rule %q command must start with an absolute executable path", rule.ID)
		}
		for _, arg := range rule.Command {
			if strings.TrimSpace(arg) == "" || strings.ContainsRune(arg, 0) {
				return fmt.Errorf("rule %q command entries must not be empty", rule.ID)
			}
		}
		if rule.TimeoutMS < 0 || rule.TimeoutMS > MaxTimeoutMS {
			return fmt.Errorf("rule %q timeout_ms must be between 0 and %d", rule.ID, MaxTimeoutMS)
		}
		if rule.WorkingDir != "" && !filepath.IsAbs(rule.WorkingDir) {
			return fmt.Errorf("rule %q working_dir must be absolute", rule.ID)
		}
		for name, value := range rule.Environment {
			if !environmentNamePattern.MatchString(name) || strings.ContainsRune(value, 0) {
				return fmt.Errorf("rule %q has invalid environment entry %q", rule.ID, name)
			}
		}
	}
	return nil
}

func (c Config) Rule(id string) (Rule, bool) {
	for _, rule := range c.Rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

func (r Rule) AllowsDoor(doorID string) bool {
	for _, allowed := range r.AllowedDoors {
		if allowed == doorID {
			return true
		}
	}
	return false
}

func (r Rule) EnvironmentList() []string {
	environment := map[string]string{"PATH": "/usr/bin:/bin"}
	for key, value := range r.Environment {
		environment[key] = value
	}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+environment[key])
	}
	return values
}
