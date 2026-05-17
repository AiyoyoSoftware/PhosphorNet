package protocol

import (
	"fmt"
	"unicode/utf8"
)

const JSONUIContractVersion = "phosphornet.ui.v1"

func ValidateRuntimeResponse(response RuntimeResponse) error {
	if response.ContractVersion != "" && response.ContractVersion != RuntimeContractVersion {
		return fmt.Errorf("unsupported runtime contract version %q", response.ContractVersion)
	}
	if response.View.Component != "screen" {
		return fmt.Errorf("runtime response view root must be screen, got %q", response.View.Component)
	}
	return ValidateUINode(response.View)
}

func ValidateUINode(node UINode) error {
	return validateUINode(node, 0, "view")
}

func validateUINode(node UINode, depth int, path string) error {
	if depth >= MaxUINodeDepth {
		return fmt.Errorf("%s exceeds max UI depth %d", path, MaxUINodeDepth)
	}

	switch node.Component {
	case "screen":
		if err := validateNoLeafFields(node, path, fieldSet{"scroll": true, "capture_keys": true, "style": true, "children": true}); err != nil {
			return err
		}
		if err := validateScroll(node.Scroll, path); err != nil {
			return err
		}
		if err := validateContainerStyle(node.Style, path); err != nil {
			return err
		}
		return validateChildren(node.Children, depth, path)
	case "header", "status":
		if err := validateNoLeafFields(node, path, fieldSet{"text": true}); err != nil {
			return err
		}
		return validateRunes(node.Text, MaxChromeTextRunes, path+".text")
	case "text", "markdown":
		if err := validateNoLeafFields(node, path, fieldSet{"text": true}); err != nil {
			return err
		}
		return validateRunes(node.Text, MaxMultilineTextRunes, path+".text")
	case "panel":
		if err := validateNoLeafFields(node, path, fieldSet{"title": true, "style": true, "children": true}); err != nil {
			return err
		}
		if err := validateRunes(node.Title, MaxChromeTextRunes, path+".title"); err != nil {
			return err
		}
		if err := validateContainerStyle(node.Style, path); err != nil {
			return err
		}
		return validateChildren(node.Children, depth, path)
	case "menu", "list", "dynamic_list":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "items": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		return validateItems(node.Items, path)
	case "input", "textarea":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "placeholder": true, "value": true, "dock": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		if err := validateRunes(node.Placeholder, MaxChromeTextRunes, path+".placeholder"); err != nil {
			return err
		}
		if err := validateRunes(node.Value, MaxMultilineTextRunes, path+".value"); err != nil {
			return err
		}
		return validateDock(node.Dock, path)
	case "button":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "text": true, "action": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		if err := validateRunes(node.Text, MaxChromeTextRunes, path+".text"); err != nil {
			return err
		}
		return validateRunes(node.Action, MaxChromeTextRunes, path+".action")
	case "checkbox":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "text": true, "checked": true, "action": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		if err := validateRunes(node.Text, MaxChromeTextRunes, path+".text"); err != nil {
			return err
		}
		return validateRunes(node.Action, MaxChromeTextRunes, path+".action")
	case "log":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "style": true, "children": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		if err := validateContainerStyle(node.Style, path); err != nil {
			return err
		}
		return validateChildren(node.Children, depth, path)
	case "grid":
		if err := validateNoLeafFields(node, path, fieldSet{"id": true, "rows": true}); err != nil {
			return err
		}
		if err := validateRequiredID(node.ID, path); err != nil {
			return err
		}
		return validateRows(node.Rows, path)
	default:
		return fmt.Errorf("%s has unsupported component %q", path, node.Component)
	}
}

type fieldSet map[string]bool

func validateNoLeafFields(node UINode, path string, allowed fieldSet) error {
	if node.ID != "" && !allowed["id"] {
		return fmt.Errorf("%s.id is not allowed on %q", path, node.Component)
	}
	if node.Text != "" && !allowed["text"] {
		return fmt.Errorf("%s.text is not allowed on %q", path, node.Component)
	}
	if node.Title != "" && !allowed["title"] {
		return fmt.Errorf("%s.title is not allowed on %q", path, node.Component)
	}
	if node.Style != nil && !allowed["style"] {
		return fmt.Errorf("%s.style is not allowed on %q", path, node.Component)
	}
	if node.Action != "" && !allowed["action"] {
		return fmt.Errorf("%s.action is not allowed on %q", path, node.Component)
	}
	if node.Scroll != "" && !allowed["scroll"] {
		return fmt.Errorf("%s.scroll is not allowed on %q", path, node.Component)
	}
	if node.CaptureKeys && !allowed["capture_keys"] {
		return fmt.Errorf("%s.capture_keys is not allowed on %q", path, node.Component)
	}
	if node.Dock != "" && !allowed["dock"] {
		return fmt.Errorf("%s.dock is not allowed on %q", path, node.Component)
	}
	if node.Value != "" && !allowed["value"] {
		return fmt.Errorf("%s.value is not allowed on %q", path, node.Component)
	}
	if node.Checked && !allowed["checked"] {
		return fmt.Errorf("%s.checked is not allowed on %q", path, node.Component)
	}
	if node.Placeholder != "" && !allowed["placeholder"] {
		return fmt.Errorf("%s.placeholder is not allowed on %q", path, node.Component)
	}
	if len(node.Items) > 0 && !allowed["items"] {
		return fmt.Errorf("%s.items is not allowed on %q", path, node.Component)
	}
	if len(node.Rows) > 0 && !allowed["rows"] {
		return fmt.Errorf("%s.rows is not allowed on %q", path, node.Component)
	}
	if len(node.Children) > 0 && !allowed["children"] {
		return fmt.Errorf("%s.children is not allowed on %q", path, node.Component)
	}
	return nil
}

func validateRequiredID(id, path string) error {
	if id == "" {
		return fmt.Errorf("%s.id is required", path)
	}
	return validateRunes(id, MaxChromeTextRunes, path+".id")
}

func validateScroll(value, path string) error {
	if value == "" || value == "bottom" {
		return nil
	}
	return fmt.Errorf("%s.scroll must be bottom when set, got %q", path, value)
}

func validateDock(value, path string) error {
	if value == "" || value == "bottom" {
		return nil
	}
	return fmt.Errorf("%s.dock must be bottom when set, got %q", path, value)
}

func validateChildren(children []UINode, depth int, path string) error {
	if len(children) > MaxUIChildren {
		return fmt.Errorf("%s.children has %d entries, max %d", path, len(children), MaxUIChildren)
	}
	for index, child := range children {
		if err := validateUINode(child, depth+1, fmt.Sprintf("%s.children[%d]", path, index)); err != nil {
			return err
		}
	}
	return nil
}

func validateItems(items []Item, path string) error {
	if len(items) > MaxUIItems {
		return fmt.Errorf("%s.items has %d entries, max %d", path, len(items), MaxUIItems)
	}
	for index, item := range items {
		if item.Label == "" {
			return fmt.Errorf("%s.items[%d].label is required", path, index)
		}
		if err := validateRunes(item.Label, MaxChromeTextRunes, fmt.Sprintf("%s.items[%d].label", path, index)); err != nil {
			return err
		}
		if err := validateRunes(item.Action, MaxChromeTextRunes, fmt.Sprintf("%s.items[%d].action", path, index)); err != nil {
			return err
		}
	}
	return nil
}

func validateRows(rows [][]string, path string) error {
	if len(rows) > MaxGridRows {
		return fmt.Errorf("%s.rows has %d entries, max %d", path, len(rows), MaxGridRows)
	}
	for rowIndex, row := range rows {
		if len(row) > MaxGridCols {
			return fmt.Errorf("%s.rows[%d] has %d cells, max %d", path, rowIndex, len(row), MaxGridCols)
		}
		for colIndex, cell := range row {
			if err := validateRunes(cell, MaxChromeTextRunes, fmt.Sprintf("%s.rows[%d][%d]", path, rowIndex, colIndex)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateContainerStyle(style *UIStyle, path string) error {
	if style == nil || style.Background == nil {
		return nil
	}
	background := style.Background
	switch background.Kind {
	case "solid":
		if !isUIHexColor(firstNonEmpty(background.Color, background.From)) {
			return fmt.Errorf("%s.style.background solid requires color or from as #rrggbb", path)
		}
	case "gradient":
		switch background.Direction {
		case "", "vertical", "horizontal", "diagonal":
		default:
			return fmt.Errorf("%s.style.background.direction %q is not supported", path, background.Direction)
		}
		if len(background.Stops) > 0 {
			if len(background.Stops) > MaxUIGradientStops {
				return fmt.Errorf("%s.style.background.stops has %d entries, max %d", path, len(background.Stops), MaxUIGradientStops)
			}
			for index, stop := range background.Stops {
				if stop.At < 0 || stop.At > 1 {
					return fmt.Errorf("%s.style.background.stops[%d].at must be between 0 and 1", path, index)
				}
				if !isUIHexColor(stop.Color) {
					return fmt.Errorf("%s.style.background.stops[%d].color must be #rrggbb", path, index)
				}
			}
			return nil
		}
		if !isUIHexColor(firstNonEmpty(background.From, background.Color)) || !isUIHexColor(firstNonEmpty(background.To, background.From, background.Color)) {
			return fmt.Errorf("%s.style.background gradient requires from/to colors or stops", path)
		}
	default:
		return fmt.Errorf("%s.style.background.kind must be solid or gradient, got %q", path, background.Kind)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func isUIHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func validateRunes(value string, limit int, path string) error {
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%s has %d runes, max %d", path, utf8.RuneCountInString(value), limit)
	}
	return nil
}
