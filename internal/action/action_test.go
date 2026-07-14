package action

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigAndRunExplicitCommand(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "actiond.toml")
	content := `
socket = "` + filepath.Join(dir, "actiond.sock") + `"
max_request_bytes = 65536
max_output_bytes = 64

[[rules]]
id = "echo-json"
allowed_doors = ["tools"]
command = ["/bin/sh", "-c", "read input; printf 'received:%s' \"$input\""]
timeout_ms = 1000
`
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	response := (Runner{Config: cfg}).Run(context.Background(), Request{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "request-1",
		RuleID:          "echo-json",
		DoorID:          "tools",
		Input:           map[string]any{"message": "hello"},
	})
	if !response.OK || response.ExitCode != 0 {
		t.Fatalf("Run() response = %#v, want successful exit", response)
	}
	if response.Stdout != `received:{"message":"hello"}` {
		t.Fatalf("Run() stdout = %q", response.Stdout)
	}
}

func TestInitCommandWritesLoadableLocalConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actiond.toml")
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"init", "--out", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Socket != filepath.Join(filepath.Dir(path), "actiond.sock") {
		t.Fatalf("Socket = %q, want config-local socket", cfg.Socket)
	}
}

func TestBundledActionDemoConfigLoadsWithStrictValidation(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "doors", "action_demo", "actiond.example.toml"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(bundled action demo) error = %v", err)
	}
	if !filepath.IsAbs(cfg.Socket) {
		t.Fatalf("bundled config socket = %q, want expanded absolute user path", cfg.Socket)
	}
	wantRules := []string{"demo-uptime", "demo-disk-usage", "demo-kernel-version"}
	for _, ruleID := range wantRules {
		rule, ok := cfg.Rule(ruleID)
		if !ok {
			t.Fatalf("bundled config missing rule %q", ruleID)
		}
		if !rule.AllowsDoor("action_demo") {
			t.Fatalf("bundled rule %q does not allow action_demo", ruleID)
		}
	}
}

func TestRunnerRejectsDoorNotAllowedByRule(t *testing.T) {
	cfg := testConfig(t, Rule{ID: "status", AllowedDoors: []string{"tools"}, Command: []string{"/bin/true"}})
	response := (Runner{Config: cfg}).Run(context.Background(), Request{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "request-1",
		RuleID:          "status",
		DoorID:          "chat",
	})
	if response.OK || !strings.Contains(response.Error, "not allowed") {
		t.Fatalf("Run() response = %#v, want denied door", response)
	}
}

func TestRunnerCapsOutputAndTimesOut(t *testing.T) {
	cfg := testConfig(t, Rule{
		ID:           "slow",
		AllowedDoors: []string{"tools"},
		Command:      []string{"/bin/sh", "-c", "printf '123456789'; sleep 1"},
		TimeoutMS:    20,
	})
	cfg.MaxOutputBytes = 4
	response := (Runner{Config: cfg}).Run(context.Background(), Request{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "request-1",
		RuleID:          "slow",
		DoorID:          "tools",
	})
	if !response.TimedOut || !response.Truncated || response.Stdout != "1234" {
		t.Fatalf("Run() response = %#v, want timeout and capped output", response)
	}
}

func TestClientServerJSONRoundTrip(t *testing.T) {
	cfg := testConfig(t, Rule{ID: "status", AllowedDoors: []string{"tools"}, Command: []string{"/bin/echo", "ready"}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverDone := make(chan error, 1)
	go func() { serverDone <- NewServer(cfg).Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(cfg.Socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actiond socket was not created")
		}
		time.Sleep(time.Millisecond)
	}
	response, err := (Client{Socket: cfg.Socket}).Execute(context.Background(), Request{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "request-1",
		RuleID:          "status",
		DoorID:          "tools",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !response.OK || response.Stdout != "ready\n" {
		t.Fatalf("Execute() response = %#v", response)
	}
	cancel()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not stop after cancellation")
	}
}

func TestValidateConfigRejectsWildcardAndRelativeCommand(t *testing.T) {
	cfg := testConfig(t)
	cfg.Rules = []Rule{{ID: "bad", AllowedDoors: []string{"*"}, Command: []string{"echo"}}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("ValidateConfig() error = nil, want unsafe rule rejection")
	}
}

func testConfig(t *testing.T, rules ...Rule) Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Socket = filepath.Join(t.TempDir(), "actiond.sock")
	cfg.Rules = rules
	return cfg
}
