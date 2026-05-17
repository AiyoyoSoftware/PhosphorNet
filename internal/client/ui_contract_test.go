package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phosphornet/internal/protocol"
)

type renderGolden struct {
	Width          int               `json:"width"`
	ViewportHeight int               `json:"viewport_height"`
	FocusedIndex   int               `json:"focused_index"`
	View           protocol.UINode   `json:"view"`
	Contains       []string          `json:"contains"`
	Focusables     []focusableGolden `json:"focusables"`
}

type focusableGolden struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Action string `json:"action"`
}

func TestJSONUIContractRenderGoldens(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "ui_contract", "v1", "render", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no render contract fixtures")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var golden renderGolden
			if err := json.Unmarshal(data, &golden); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			if err := protocol.ValidateUINode(golden.View); err != nil {
				t.Fatalf("fixture does not satisfy protocol contract: %v", err)
			}

			rendered, state := RenderComponents(golden.View, renderOptions{
				Width:          golden.Width,
				ViewportHeight: golden.ViewportHeight,
				FocusedIndex:   golden.FocusedIndex,
				ItemSelection:  map[string]int{},
				InputValues:    map[string]string{},
			})
			plain := stripANSISequences(rendered)
			for _, expected := range golden.Contains {
				if !strings.Contains(plain, expected) {
					t.Fatalf("rendered output missing %q:\n%s", expected, plain)
				}
			}
			if len(state.focusables) != len(golden.Focusables) {
				t.Fatalf("focusable count = %d, want %d: %#v", len(state.focusables), len(golden.Focusables), state.focusables)
			}
			for index, expected := range golden.Focusables {
				actual := state.focusables[index]
				if string(actual.Kind) != expected.Kind || actual.ID != expected.ID || actual.Action != expected.Action {
					t.Fatalf("focusable[%d] = %#v, want %#v", index, actual, expected)
				}
			}
		})
	}
}
