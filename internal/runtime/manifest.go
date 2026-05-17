package runtime

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	DoorSettingTypeString   = "string"
	DoorSettingTypeTextarea = "textarea"
	DoorSettingTypeBool     = "bool"
	DoorSettingTypeInt      = "int"
	DoorSettingTypeSelect   = "select"
	DoorSettingTypeMarkdown = "markdown"
)

var doorSettingNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

type DoorManifest struct {
	ID           string                           `toml:"id"`
	Name         string                           `toml:"name"`
	Entry        string                           `toml:"entry"`
	Command      []string                         `toml:"command"`
	Runtime      string                           `toml:"runtime"`
	Visibility   string                           `toml:"visibility"`
	Access       string                           `toml:"access"`
	Allowlist    []string                         `toml:"allowlist"`
	Permissions  []string                         `toml:"permissions"`
	Capabilities []string                         `toml:"capabilities"`
	Isolation    DoorIsolationConfig              `toml:"isolation"`
	Sandbox      LuaSandboxConfig                 `toml:"sandbox"`
	Settings     map[string]DoorSettingDefinition `toml:"settings"`
	Dir          string                           `toml:"-"`
}

type DoorIsolationConfig struct {
	Mode      string  `toml:"mode"`
	Image     string  `toml:"image"`
	Network   string  `toml:"network"`
	ReadOnly  *bool   `toml:"read_only"`
	TimeoutMS int     `toml:"timeout_ms"`
	Memory    string  `toml:"memory"`
	CPUs      float64 `toml:"cpus"`
	PidsLimit int     `toml:"pids_limit"`
}

type DoorSettingDefinition struct {
	Type    string   `toml:"type"`
	Label   string   `toml:"label"`
	Default any      `toml:"default"`
	Options []string `toml:"options"`
}

func LoadDoorManifest(path string) (DoorManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DoorManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest DoorManifest
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return DoorManifest{}, protocolManifestError(fmt.Errorf("parse manifest: %w", err))
	}
	manifest.Dir = filepath.Dir(path)
	manifest.Capabilities = NormalizeCapabilities(manifest.Capabilities, manifest.Permissions)
	normalizeDoorSettings(&manifest)
	normalizeDoorIsolation(&manifest)
	if err := validateDoorManifest(manifest); err != nil {
		return DoorManifest{}, protocolManifestError(fmt.Errorf("validate manifest %q: %w", path, err))
	}
	return manifest, nil
}

func LoadDoorManifests(root string) ([]DoorManifest, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve doors root: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve doors root symlinks: %w", err)
	}

	pattern := filepath.Join(rootAbs, "*", "manifest.toml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob manifests: %w", err)
	}

	manifests := make([]DoorManifest, 0, len(paths))
	seenIDs := map[string]string{}
	for _, path := range paths {
		manifest, err := LoadDoorManifest(path)
		if err != nil {
			return nil, err
		}
		manifestDirReal, err := filepath.EvalSymlinks(manifest.Dir)
		if err != nil {
			err := fmt.Errorf("resolve manifest dir for %q: %w", manifest.ID, err)
			return nil, protocolManifestError(err)
		}
		if !pathWithinRoot(rootReal, manifestDirReal) {
			err := fmt.Errorf("door %q directory escapes doors root", manifest.ID)
			return nil, protocolManifestError(err)
		}
		if strings.TrimSpace(manifest.Entry) != "" {
			if _, err := resolveDoorEntryPath(rootReal, manifest); err != nil {
				return nil, protocolManifestError(err)
			}
		}
		if previousPath, exists := seenIDs[manifest.ID]; exists {
			err := fmt.Errorf("duplicate door id %q in %s and %s", manifest.ID, previousPath, path)
			return nil, protocolManifestError(err)
		}
		seenIDs[manifest.ID] = path
		manifests = append(manifests, manifest)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].ID < manifests[j].ID
	})
	return manifests, nil
}

func validateDoorManifest(manifest DoorManifest) error {
	normalizeDoorIsolation(&manifest)
	if strings.TrimSpace(manifest.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("name is required")
	}
	runtimeName := normalizeRuntimeName(manifest)
	hasPodmanImage := runtimeName == "stdio" && strings.EqualFold(strings.TrimSpace(manifest.Isolation.Mode), "podman") && strings.TrimSpace(manifest.Isolation.Image) != ""
	if strings.TrimSpace(manifest.Entry) == "" && !(runtimeName == "stdio" && (len(manifest.Command) > 0 || hasPodmanImage)) {
		return fmt.Errorf("entry is required")
	}
	if filepath.IsAbs(manifest.Entry) {
		return fmt.Errorf("entry must be relative to the door directory")
	}
	if len(manifest.Command) > 0 {
		if strings.TrimSpace(manifest.Command[0]) == "" {
			return fmt.Errorf("command executable is required")
		}
		for _, arg := range manifest.Command {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("command entries must not be empty")
			}
		}
	}
	if err := ValidateRuntimeName(manifest.Runtime); err != nil {
		return err
	}
	if runtimeName == "stdio" && len(manifest.Command) == 0 && !hasPodmanImage {
		return fmt.Errorf("stdio runtime requires command or podman isolation image")
	}
	switch strings.ToLower(strings.TrimSpace(manifest.Visibility)) {
	case "", "public", "private", "hidden":
	default:
		return fmt.Errorf("unknown visibility %q", manifest.Visibility)
	}
	switch strings.ToLower(strings.TrimSpace(manifest.Access)) {
	case "", "public", "invite_only", "admin":
	default:
		return fmt.Errorf("unknown access %q", manifest.Access)
	}
	if err := ValidateLuaSandboxConfig(manifest.Sandbox); err != nil {
		return fmt.Errorf("sandbox: %w", err)
	}
	if err := ValidateDoorIsolationConfig(runtimeName, manifest.Isolation); err != nil {
		return fmt.Errorf("isolation: %w", err)
	}
	if err := ValidateCapabilities(manifest.Capabilities); err != nil {
		return err
	}
	if err := validateDoorSettings(manifest.Settings); err != nil {
		return err
	}
	return nil
}

func normalizeDoorIsolation(manifest *DoorManifest) {
	runtimeName := normalizeRuntimeName(*manifest)
	if stdioRuntimeUsesIsolation(runtimeName) && strings.TrimSpace(manifest.Isolation.Mode) == "" {
		manifest.Isolation.Mode = IsolationModePodman
	}
}

func normalizeDoorSettings(manifest *DoorManifest) {
	for name, setting := range manifest.Settings {
		setting.Type = strings.ToLower(strings.TrimSpace(setting.Type))
		setting.Label = strings.TrimSpace(setting.Label)
		manifest.Settings[name] = setting
	}
}

func validateDoorSettings(settings map[string]DoorSettingDefinition) error {
	for name, setting := range settings {
		if !doorSettingNamePattern.MatchString(name) {
			return fmt.Errorf("setting %q has invalid name; use letters, numbers, and underscores, starting with a letter or underscore", name)
		}
		switch setting.Type {
		case DoorSettingTypeString, DoorSettingTypeTextarea, DoorSettingTypeBool, DoorSettingTypeInt, DoorSettingTypeSelect, DoorSettingTypeMarkdown:
		case "":
			return fmt.Errorf("setting %q type is required", name)
		default:
			return fmt.Errorf("setting %q has unknown type %q", name, setting.Type)
		}
		if setting.Type == DoorSettingTypeSelect && len(setting.Options) == 0 {
			return fmt.Errorf("setting %q select options are required", name)
		}
		if _, ok := CoerceDoorSettingValue(setting, setting.Default); !ok {
			return fmt.Errorf("setting %q default does not match type %q", name, setting.Type)
		}
	}
	return nil
}

func DoorSettingDefaults(manifest DoorManifest) map[string]any {
	defaults := make(map[string]any, len(manifest.Settings))
	for name, setting := range manifest.Settings {
		if value, ok := CoerceDoorSettingValue(setting, setting.Default); ok {
			defaults[name] = value
		}
	}
	return defaults
}

func ResolveDoorSettings(manifest DoorManifest, overrides map[string]any) map[string]any {
	values := DoorSettingDefaults(manifest)
	for name, raw := range overrides {
		setting, ok := manifest.Settings[name]
		if !ok {
			continue
		}
		if value, ok := CoerceDoorSettingValue(setting, raw); ok {
			values[name] = value
		}
	}
	return values
}

func CoerceDoorSettingValue(setting DoorSettingDefinition, value any) (any, bool) {
	if value == nil {
		return doorSettingZeroValue(setting), true
	}
	switch setting.Type {
	case DoorSettingTypeString, DoorSettingTypeTextarea, DoorSettingTypeMarkdown:
		switch v := value.(type) {
		case string:
			return v, true
		default:
			return fmt.Sprint(v), true
		}
	case DoorSettingTypeBool:
		switch v := value.(type) {
		case bool:
			return v, true
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err == nil {
				return parsed, true
			}
		}
	case DoorSettingTypeInt:
		switch v := value.(type) {
		case int:
			return v, true
		case int64:
			return int(v), true
		case float64:
			if float64(int(v)) == v {
				return int(v), true
			}
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil {
				return parsed, true
			}
		}
	case DoorSettingTypeSelect:
		selected := fmt.Sprint(value)
		for _, option := range setting.Options {
			if selected == option {
				return selected, true
			}
		}
	}
	return nil, false
}

func doorSettingZeroValue(setting DoorSettingDefinition) any {
	switch setting.Type {
	case DoorSettingTypeBool:
		return false
	case DoorSettingTypeInt:
		return 0
	case DoorSettingTypeSelect:
		if len(setting.Options) > 0 {
			return setting.Options[0]
		}
	}
	return ""
}

func pathWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
