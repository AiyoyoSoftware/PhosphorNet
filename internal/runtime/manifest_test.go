package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoorManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"lobby\"\nname = \"Lobby\"\nentry = \"app.py\"\nvisibility = \"private\"\naccess = \"invite_only\"\nallowlist = [\"abc\"]\ncapabilities = [\"state:user:read\", \"state:user:write\"]\npermissions = [\"raw_keys\"]\n\n[settings.motd]\ntype = \"textarea\"\nlabel = \"Message of the day\"\ndefault = \"Welcome\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadDoorManifest(path)
	if err != nil {
		t.Fatalf("LoadDoorManifest() error = %v", err)
	}
	if manifest.ID != "lobby" {
		t.Fatalf("manifest.ID = %q, want %q", manifest.ID, "lobby")
	}
	if manifest.Visibility != "private" {
		t.Fatalf("manifest.Visibility = %q, want private", manifest.Visibility)
	}
	if manifest.Access != "invite_only" {
		t.Fatalf("manifest.Access = %q, want invite_only", manifest.Access)
	}
	if len(manifest.Allowlist) != 1 || manifest.Allowlist[0] != "abc" {
		t.Fatalf("manifest.Allowlist = %#v, want [abc]", manifest.Allowlist)
	}
	if !HasCapability(manifest.Capabilities, CapabilityStateUserRead) || !HasCapability(manifest.Capabilities, CapabilityCaptureKeys) {
		t.Fatalf("manifest.Capabilities = %#v, want explicit and legacy-mapped capabilities", manifest.Capabilities)
	}
	if manifest.Settings["motd"].Type != DoorSettingTypeTextarea {
		t.Fatalf("manifest.Settings[motd].Type = %q, want textarea", manifest.Settings["motd"].Type)
	}
	if got := DoorSettingDefaults(manifest)["motd"]; got != "Welcome" {
		t.Fatalf("DoorSettingDefaults()[motd] = %#v, want Welcome", got)
	}
}

func TestLoadDoorManifestRejectsInvalidSettingDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"lobby\"\nname = \"Lobby\"\nentry = \"app.lua\"\n\n[settings.board_size]\ntype = \"int\"\ndefault = \"large\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadDoorManifest(path); err == nil {
		t.Fatal("LoadDoorManifest() error = nil, want invalid setting default error")
	}
}

func TestLoadDoorManifestAllowsStdioCommandWithoutEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"echo\"\nname = \"Echo\"\nruntime = \"stdio\"\ncommand = [\"python3\", \"app.py\"]\n\n[isolation]\nmode = \"host\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadDoorManifest(path)
	if err != nil {
		t.Fatalf("LoadDoorManifest() error = %v", err)
	}
	if manifest.Entry != "" {
		t.Fatalf("manifest.Entry = %q, want empty", manifest.Entry)
	}
	if len(manifest.Command) != 2 || manifest.Command[0] != "python3" || manifest.Command[1] != "app.py" {
		t.Fatalf("manifest.Command = %#v, want python3 app.py", manifest.Command)
	}
	if manifest.Isolation.Mode != "host" {
		t.Fatalf("manifest.Isolation.Mode = %q, want host", manifest.Isolation.Mode)
	}
}

func TestLoadDoorManifestRejectsStdioWithoutPodmanImageUnlessHostOptOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"echo\"\nname = \"Echo\"\nruntime = \"stdio\"\ncommand = [\"python3\", \"app.py\"]\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadDoorManifest(path); err == nil {
		t.Fatal("LoadDoorManifest() error = nil, want missing podman image error")
	}
}

func TestLoadDoorManifestDefaultsStdioIsolationToPodman(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"weather\"\nname = \"Weather\"\nruntime = \"stdio\"\n\n[isolation]\nimage = \"localhost/phosphornet/weather-door:0.1.0\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadDoorManifest(path)
	if err != nil {
		t.Fatalf("LoadDoorManifest() error = %v", err)
	}
	if manifest.Isolation.Mode != "podman" {
		t.Fatalf("manifest.Isolation.Mode = %q, want podman", manifest.Isolation.Mode)
	}
	if manifest.Isolation.Image != "localhost/phosphornet/weather-door:0.1.0" {
		t.Fatalf("manifest.Isolation.Image = %q, want weather image", manifest.Isolation.Image)
	}
}

func TestLoadDoorManifestRejectsPythonRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"legacy_python\"\nname = \"Legacy Python\"\nruntime = \"python\"\nentry = \"app.py\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadDoorManifest(path); err == nil {
		t.Fatal("LoadDoorManifest() error = nil, want unknown runtime error")
	}
}

func TestLoadDoorManifestAllowsPodmanStdioImageWithoutCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"weather\"\nname = \"Weather\"\nruntime = \"stdio\"\n\n[isolation]\nmode = \"podman\"\nimage = \"localhost/phosphornet/weather-door:0.1.0\"\nnetwork = \"none\"\nread_only = true\ntimeout_ms = 1500\nmemory = \"128m\"\ncpus = 0.25\npids_limit = 64\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadDoorManifest(path)
	if err != nil {
		t.Fatalf("LoadDoorManifest() error = %v", err)
	}
	if manifest.Isolation.Mode != "podman" {
		t.Fatalf("manifest.Isolation.Mode = %q, want podman", manifest.Isolation.Mode)
	}
	if manifest.Isolation.Image != "localhost/phosphornet/weather-door:0.1.0" {
		t.Fatalf("manifest.Isolation.Image = %q, want weather image", manifest.Isolation.Image)
	}
	if manifest.Isolation.ReadOnly == nil || !*manifest.Isolation.ReadOnly {
		t.Fatalf("manifest.Isolation.ReadOnly = %#v, want true", manifest.Isolation.ReadOnly)
	}
}

func TestLoadDoorManifestRejectsPodmanIsolationWithoutImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	content := []byte("id = \"weather\"\nname = \"Weather\"\nruntime = \"stdio\"\ncommand = [\"python3\", \"app.py\"]\n\n[isolation]\nmode = \"podman\"\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadDoorManifest(path); err == nil {
		t.Fatal("LoadDoorManifest() error = nil, want missing podman image error")
	}
}

func TestNormalizeRuntimeNameDefaultsToLua(t *testing.T) {
	got := normalizeRuntimeName(DoorManifest{Entry: "app"})
	if got != "lua" {
		t.Fatalf("normalizeRuntimeName() = %q, want lua", got)
	}
}

func TestResolveDoorEntryPathRejectsEscapes(t *testing.T) {
	_, err := resolveDoorEntryPath(t.TempDir(), DoorManifest{
		ID:    "lobby",
		Entry: "../outside.lua",
	})
	if err == nil {
		t.Fatal("resolveDoorEntryPath() error = nil, want escape error")
	}
}

func TestLoadDoorManifestsRejectsDuplicateIDs(t *testing.T) {
	root := t.TempDir()
	for _, doorDir := range []string{"lobby-a", "lobby-b"} {
		dir := filepath.Join(root, doorDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "app.lua"), []byte("function view(ctx) return { component = 'screen', children = {} } end"), 0o644); err != nil {
			t.Fatalf("WriteFile(app.lua) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte("id = \"lobby\"\nname = \"Lobby\"\nentry = \"app.lua\"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(manifest.toml) error = %v", err)
		}
	}

	if _, err := LoadDoorManifests(root); err == nil {
		t.Fatal("LoadDoorManifests() error = nil, want duplicate door id rejection")
	}
}

func TestLoadDoorManifestsRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	doorDir := filepath.Join(root, "lobby")
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "evil.lua")
	if err := os.WriteFile(outsidePath, []byte("return {}"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(doorDir, "app.lua")); err != nil {
		t.Skipf("Symlink() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorDir, "manifest.toml"), []byte("id = \"lobby\"\nname = \"Lobby\"\nentry = \"app.lua\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest.toml) error = %v", err)
	}

	if _, err := LoadDoorManifests(root); err == nil {
		t.Fatal("LoadDoorManifests() error = nil, want symlink escape rejection")
	}
}

func TestBundledDoorManifestsLoad(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs(doors) error = %v", err)
	}

	manifests, err := LoadDoorManifests(root)
	if err != nil {
		t.Fatalf("LoadDoorManifests(bundled) error = %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("LoadDoorManifests(bundled) returned no manifests")
	}
}
