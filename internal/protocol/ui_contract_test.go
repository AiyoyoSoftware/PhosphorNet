package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONUIContractSchemaIsOwnedByProtocolPackage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("schema", "json-ui-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	raw, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("schema missing $defs")
	}
	node, ok := raw["uiNode"].(map[string]any)
	if !ok {
		t.Fatal("schema missing $defs.uiNode")
	}
	serialized := string(data)
	for _, component := range []string{
		"screen", "header", "text", "markdown", "status", "panel", "menu", "list",
		"dynamic_list", "input", "textarea", "button", "checkbox", "log", "grid",
	} {
		if !strings.Contains(serialized, `"`+component+`"`) {
			t.Fatalf("schema does not mention component %q", component)
		}
	}
	for _, limit := range []string{"64", "20", "12", "8", "128", "2048"} {
		if !strings.Contains(serialized, limit) {
			t.Fatalf("schema does not mention protocol limit %s", limit)
		}
	}
	if _, ok := node["oneOf"]; !ok {
		t.Fatal("schema uiNode is not component-specific")
	}
}

func TestGoldenUIContractValidExamples(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "ui_contract", "v1", "valid", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no valid UI contract fixtures")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			node := readFixtureNode(t, path)
			if err := ValidateUINode(node); err != nil {
				t.Fatalf("ValidateUINode() error = %v", err)
			}
			response := RuntimeResponse{
				ContractVersion: RuntimeContractVersion,
				View:            node,
			}
			if err := ValidateRuntimeResponse(response); err != nil {
				t.Fatalf("ValidateRuntimeResponse() error = %v", err)
			}
		})
	}
}

func TestGoldenUIContractInvalidExamples(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "ui_contract", "v1", "invalid", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no invalid UI contract fixtures")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			node := readFixtureNode(t, path)
			if err := ValidateUINode(node); err == nil {
				t.Fatal("ValidateUINode() error = nil, want invalid fixture rejected")
			}
		})
	}
}

func TestUIContractGeneratedLimitBoundaries(t *testing.T) {
	children := make([]UINode, 0, MaxUIChildren)
	for index := 0; index < MaxUIChildren; index++ {
		children = append(children, Text("ok"))
	}
	items := make([]Item, 0, MaxUIItems)
	for index := 0; index < MaxUIItems; index++ {
		items = append(items, Item{Label: "choice", Action: "choose"})
	}
	rows := make([][]string, 0, MaxGridRows)
	for row := 0; row < MaxGridRows; row++ {
		cells := make([]string, 0, MaxGridCols)
		for col := 0; col < MaxGridCols; col++ {
			cells = append(cells, "x")
		}
		rows = append(rows, cells)
	}
	stops := make([]UIGradientStop, 0, MaxUIGradientStops)
	for index := 0; index < MaxUIGradientStops; index++ {
		stops = append(stops, UIGradientStop{At: float64(index) / float64(MaxUIGradientStops-1), Color: "#111111"})
	}

	valid := Screen(children...)
	valid.Style = &UIStyle{Background: &UIBackground{Kind: "gradient", Direction: "vertical", Stops: stops}}
	valid.Children = append(valid.Children[:0], append([]UINode{
		Menu("max-items", items...),
		Grid("max-grid", rows),
	}, valid.Children[2:]...)...)
	if err := ValidateUINode(valid); err != nil {
		t.Fatalf("max boundary UI should validate: %v", err)
	}

	tooManyChildren := Screen(append(children, Text("too much"))...)
	if err := ValidateUINode(tooManyChildren); err == nil {
		t.Fatal("too many children validated, want rejection")
	}

	tooManyItems := Menu("too-many-items", append(items, Item{Label: "extra"})...)
	if err := ValidateUINode(tooManyItems); err == nil {
		t.Fatal("too many items validated, want rejection")
	}

	tooManyRows := Grid("too-many-rows", append(rows, []string{"extra"}))
	if err := ValidateUINode(tooManyRows); err == nil {
		t.Fatal("too many grid rows validated, want rejection")
	}

	tooManyStops := Screen()
	tooManyStops.Style = &UIStyle{Background: &UIBackground{
		Kind:      "gradient",
		Direction: "vertical",
		Stops:     append(stops, UIGradientStop{At: 1, Color: "#222222"}),
	}}
	if err := ValidateUINode(tooManyStops); err == nil {
		t.Fatal("too many gradient stops validated, want rejection")
	}
}

func TestRuntimeResponseRequiresScreenRoot(t *testing.T) {
	err := ValidateRuntimeResponse(RuntimeResponse{
		ContractVersion: RuntimeContractVersion,
		View:            Text("not a screen"),
	})
	if err == nil {
		t.Fatal("ValidateRuntimeResponse() error = nil, want non-screen root rejected")
	}
}

func readFixtureNode(t *testing.T, path string) UINode {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var node UINode
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return node
}
