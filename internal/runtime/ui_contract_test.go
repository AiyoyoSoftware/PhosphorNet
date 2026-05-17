package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"phosphornet/internal/protocol"
)

func TestLuaSDKMatchesJSONUIContractGolden(t *testing.T) {
	doorsRoot := t.TempDir()
	doorDir := filepath.Join(doorsRoot, "contract_lua")
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(doorDir, "app.lua"), `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.header("LUA SDK"),
    ui.panel("Helpers", {
      ui.text("ui.text output"),
      ui.markdown("**ui.markdown** output"),
      ui.menu("lua-menu", {
        ui.item("Record Visit", "record_visit"),
      }),
      ui.input("lua-input", "name", "Ada"),
      ui.textarea("lua-textarea", "note", "hello"),
      ui.button("lua-button", "Save", "save"),
      ui.checkbox("lua-checkbox", "Enabled", false, "toggle"),
      ui.grid("lua-grid", {
        { "x", "o" },
        { "o", "x" },
      }),
    }),
    ui.log("lua-log", {
      ui.status("Lua helper corpus row"),
    }),
  })
end
`)

	response, err := InvokeDoorView(context.Background(), doorsRoot, DoorManifest{
		ID:      "contract_lua",
		Name:    "Contract Lua",
		Runtime: "lua",
		Entry:   "app.lua",
		Dir:     doorDir,
	}, protocol.RuntimeContext{})
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	assertMatchesUIGolden(t, response.View, filepath.Join("..", "protocol", "testdata", "ui_contract", "v1", "valid", "sdk_lua.json"))
}

func TestPythonSDKMatchesJSONUIContractGolden(t *testing.T) {
	doorsRoot := t.TempDir()
	doorDir := filepath.Join(doorsRoot, "contract_python")
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(doorDir, "app.py"), `
import asyncio

from phosphornet import run_module, ui


async def view(ctx):
    return ui.screen(
        [
            ui.header("PYTHON SDK"),
            ui.log(
                "python-log",
                [
                    ui.text("<ada> hello"),
                    ui.markdown("**ui.markdown** output"),
                ],
            ),
            ui.input("chat-message", "/msg #station", dock="bottom"),
            ui.panel(
                "Actions",
                [
                    ui.dynamic_list(
                        "python-dynamic-list",
                        [
                            ui.item("One", action="one"),
                            ui.item("Two", action="two"),
                        ],
                    ),
                    ui.checkbox("python-checkbox", "Enabled", True, "toggle"),
                    ui.button("python-button", "Submit", "submit"),
                ],
            ),
        ],
        scroll="bottom",
    )


if __name__ == "__main__":
    asyncio.run(run_module(globals()))
`)
	includePythonSDK(t, doorsRoot, "contract_python")

	response, err := InvokeDoorView(context.Background(), doorsRoot, DoorManifest{
		ID:        "contract_python",
		Name:      "Contract Python",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Dir:       doorDir,
		Isolation: hostIsolation(),
	}, protocol.RuntimeContext{})
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	assertMatchesUIGolden(t, response.View, filepath.Join("..", "protocol", "testdata", "ui_contract", "v1", "valid", "sdk_python.json"))
}

func assertMatchesUIGolden(t *testing.T, actual protocol.UINode, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var expected protocol.UINode
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("SDK output does not match %s\nactual: %#v\nwant: %#v", path, actual, expected)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
