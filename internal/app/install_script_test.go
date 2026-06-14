package app

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptClientModeInstallsOnlyClientBinary(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	output := runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--client")
	if !strings.Contains(output, "PhosphorNet client install complete") {
		t.Fatalf("install output missing completion line:\n%s", output)
	}
	assertFileExists(t, filepath.Join(binDir, "phosphor"))
	assertFileMissing(t, filepath.Join(binDir, "phosphord"))
	assertFileMissing(t, filepath.Join(binDir, "switchboard"))
	assertFileMissing(t, filepath.Join(configDir, "node.toml"))
}

func TestInstallScriptDefaultReleaseBasePointsAtGitHubRepository(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatalf("ReadFile(install.sh) error = %v", err)
	}
	want := "https://github.com/AiyoyoSoftware/PhosphorNet/releases/download"
	if !strings.Contains(string(data), want) {
		t.Fatalf("install.sh does not contain default release base %q", want)
	}
}

func TestInstallScriptDefaultSystemPathsRequireRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can install to default system paths")
	}
	root := repoRoot(t)
	cmd := exec.Command("sh", filepath.Join(root, "install.sh"), "--client")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("install.sh --client with default system paths succeeded as non-root, want safe failure")
	}
	if !strings.Contains(string(output), "Re-run as root") {
		t.Fatalf("install output = %q, want root guidance", string(output))
	}
}

func TestInstallScriptNodeModeInstallsNodeAssetsAndConfig(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	output := runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--node")
	if !strings.Contains(output, "PhosphorNet node install complete") {
		t.Fatalf("install output missing completion line:\n%s", output)
	}
	assertFileExists(t, filepath.Join(binDir, "phosphord"))
	assertFileMissing(t, filepath.Join(binDir, "phosphor"))
	assertFileMissing(t, filepath.Join(binDir, "switchboard"))
	assertFileExists(t, filepath.Join(shareDir, "doors", "lobby", "manifest.toml"))
	configPath := filepath.Join(configDir, "node.toml")
	assertFileExists(t, configPath)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if !strings.Contains(string(configData), "system_paths = true") {
		t.Fatalf("node config was not created by fake phosphord --system-paths:\n%s", string(configData))
	}
}

func TestInstallScriptSudoUserDefaultAdminPassportUsesOperatorHome(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")
	operatorHome := t.TempDir()

	cmd := exec.Command("sh", filepath.Join(root, "install.sh"), "--node")
	cmd.Env = append(os.Environ(),
		"HOME=/root",
		"PATH="+fakeGetentDir(t, "stationop", operatorHome)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SUDO_USER=stationop",
		"SUDO_UID="+os.Getenv("UID"),
		"SUDO_GID="+os.Getenv("GID"),
		"PHOSPHORNET_ARTIFACT_DIR="+artifactDir,
		"PHOSPHORNET_BIN_DIR="+binDir,
		"PHOSPHORNET_SHARE_DIR="+shareDir,
		"PHOSPHORNET_CONFIG_DIR="+configDir,
		"PHOSPHORNET_STATE_DIR="+stateDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh sudo-user default passport error = %v\n%s", err, string(output))
	}

	configData, err := os.ReadFile(filepath.Join(configDir, "node.toml"))
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	want := "admin_passport = \"" + filepath.Join(operatorHome, ".config", "phosphornet", "passport.toml") + "\""
	if !strings.Contains(string(configData), want) {
		t.Fatalf("config = %q, want %q", string(configData), want)
	}
}

func TestInstallScriptFullModeInstallsAllBinaries(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--full")
	assertFileExists(t, filepath.Join(binDir, "phosphor"))
	assertFileExists(t, filepath.Join(binDir, "phosphord"))
	assertFileExists(t, filepath.Join(binDir, "switchboard"))
	assertFileExists(t, filepath.Join(shareDir, "doors", "lobby", "manifest.toml"))
	assertFileExists(t, filepath.Join(configDir, "node.toml"))
}

func TestInstallScriptInstallsFromReleaseArchiveURL(t *testing.T) {
	root := repoRoot(t)
	archivePath := fakeReleaseArchive(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	cmd := exec.Command("sh", filepath.Join(root, "install.sh"), "--node")
	cmd.Env = append(os.Environ(),
		"PHOSPHORNET_ARTIFACT_URL=file://"+archivePath,
		"PHOSPHORNET_BIN_DIR="+binDir,
		"PHOSPHORNET_SHARE_DIR="+shareDir,
		"PHOSPHORNET_CONFIG_DIR="+configDir,
		"PHOSPHORNET_STATE_DIR="+stateDir,
		"PHOSPHORNET_ADMIN_PASSPORT="+filepath.Join(t.TempDir(), "passport.toml"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh release archive error = %v\n%s", err, string(output))
	}

	assertFileExists(t, filepath.Join(binDir, "phosphord"))
	assertFileExists(t, filepath.Join(shareDir, "doors", "lobby", "manifest.toml"))
	assertFileExists(t, filepath.Join(configDir, "node.toml"))
}

func TestInstallScriptUninstallRemovesInstalledFilesButKeepsState(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--full")
	dbPath := filepath.Join(stateDir, "phosphornet.db")
	if err := os.WriteFile(dbPath, []byte("station memory"), 0o600); err != nil {
		t.Fatalf("WriteFile(db) error = %v", err)
	}

	output := runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--uninstall", "--full")
	if !strings.Contains(output, "kept "+filepath.Join(configDir, "node.toml")) {
		t.Fatalf("uninstall output did not mention kept state:\n%s", output)
	}
	assertFileMissing(t, filepath.Join(binDir, "phosphor"))
	assertFileMissing(t, filepath.Join(binDir, "phosphord"))
	assertFileMissing(t, filepath.Join(binDir, "switchboard"))
	assertFileMissing(t, filepath.Join(shareDir, "doors"))
	assertFileExists(t, filepath.Join(configDir, "node.toml"))
	assertFileExists(t, dbPath)
}

func TestInstallScriptUninstallPurgeRemovesConfigAndState(t *testing.T) {
	root := repoRoot(t)
	artifactDir := fakeArtifactDir(t)
	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	shareDir := filepath.Join(installRoot, "share", "phosphornet")
	configDir := filepath.Join(installRoot, "etc", "phosphornet")
	stateDir := filepath.Join(installRoot, "var", "lib", "phosphornet")

	runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--node")
	dbPath := filepath.Join(stateDir, "phosphornet.db")
	if err := os.WriteFile(dbPath, []byte("station memory"), 0o600); err != nil {
		t.Fatalf("WriteFile(db) error = %v", err)
	}

	runInstallScript(t, root, artifactDir, binDir, shareDir, configDir, stateDir, "--uninstall", "--purge", "--node")
	assertFileMissing(t, filepath.Join(binDir, "phosphord"))
	assertFileMissing(t, filepath.Join(shareDir, "doors"))
	assertFileMissing(t, filepath.Join(configDir, "node.toml"))
	assertFileMissing(t, dbPath)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs(repo root) error = %v", err)
	}
	return root
}

func fakeArtifactDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "phosphor"), "#!/bin/sh\necho phosphor \"$@\"\n")
	writeExecutable(t, filepath.Join(dir, "switchboard"), "#!/bin/sh\necho switchboard \"$@\"\n")
	writeExecutable(t, filepath.Join(dir, "phosphord"), `#!/bin/sh
if [ "$1" = "init" ]; then
	out=""
	admin_passport=""
	system_paths=false
	while [ "$#" -gt 0 ]; do
		case "$1" in
			--out)
				shift
				out="$1"
				;;
			--admin-passport)
				shift
				admin_passport="$1"
				;;
			--system-paths)
				system_paths=true
				;;
		esac
		shift
	done
	mkdir -p "$(dirname "$out")"
	printf 'system_paths = %s\nadmin_passport = "%s"\n' "$system_paths" "$admin_passport" > "$out"
	exit 0
fi
echo phosphord "$@"
`)
	doorDir := filepath.Join(dir, "doors", "lobby")
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(door) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorDir, "manifest.toml"), []byte("id = \"lobby\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	return dir
}

func fakeGetentDir(t *testing.T, username, home string) string {
	t.Helper()
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "getent"), "#!/bin/sh\nif [ \"$1\" = \"passwd\" ] && [ \"$2\" = \""+username+"\" ]; then\n  echo '"+username+":x:1000:1000:Station Operator:"+home+":/bin/sh'\nfi\n")
	return dir
}

func fakeReleaseArchive(t *testing.T) string {
	t.Helper()
	artifactDir := fakeArtifactDir(t)
	archivePath := filepath.Join(t.TempDir(), "phosphornet_linux_amd64.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create(archive) error = %v", err)
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	packageRoot := "phosphornet_linux_amd64"
	addTarFile(t, tw, artifactDir, "phosphord", packageRoot+"/phosphord", 0o755)
	addTarFile(t, tw, artifactDir, filepath.Join("doors", "lobby", "manifest.toml"), packageRoot+"/doors/lobby/manifest.toml", 0o644)
	return archivePath
}

func addTarFile(t *testing.T, tw *tar.Writer, root, localPath, archivePath string, mode int64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, localPath))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", localPath, err)
	}
	header := &tar.Header{
		Name: archivePath,
		Mode: mode,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader(%s) error = %v", archivePath, err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("Write(%s) error = %v", archivePath, err)
	}
}

func runInstallScript(t *testing.T, root, artifactDir, binDir, shareDir, configDir, stateDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join(root, "install.sh")}, args...)...)
	cmd.Env = append(os.Environ(),
		"PHOSPHORNET_ARTIFACT_DIR="+artifactDir,
		"PHOSPHORNET_BIN_DIR="+binDir,
		"PHOSPHORNET_SHARE_DIR="+shareDir,
		"PHOSPHORNET_CONFIG_DIR="+configDir,
		"PHOSPHORNET_STATE_DIR="+stateDir,
		"PHOSPHORNET_ADMIN_PASSPORT="+filepath.Join(t.TempDir(), "passport.toml"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh %v error = %v\n%s", args, err, string(output))
	}
	return string(output)
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat error = %v", path, err)
	}
}
