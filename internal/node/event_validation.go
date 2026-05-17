package node

import (
	"fmt"

	"phosphornet/internal/protocol"
)

type renderEventPolicy struct {
	captureKeys bool
	components  map[string]componentEventPolicy
}

type componentEventPolicy struct {
	component string
	actions   map[string]bool
}

func buildRenderEventPolicy(view protocol.UINode) (renderEventPolicy, error) {
	policy := renderEventPolicy{
		captureKeys: view.Component == "screen" && view.CaptureKeys,
		components:  map[string]componentEventPolicy{},
	}
	if err := addNodeToRenderPolicy(&policy, view); err != nil {
		return renderEventPolicy{}, err
	}
	return policy, nil
}

func addNodeToRenderPolicy(policy *renderEventPolicy, node protocol.UINode) error {
	switch node.Component {
	case "screen", "panel", "log":
		for _, child := range node.Children {
			if err := addNodeToRenderPolicy(policy, child); err != nil {
				return err
			}
		}
	case "button", "checkbox":
		return addComponentPolicy(policy, node.ID, node.Component, map[string]bool{node.Action: true})
	case "menu", "list", "dynamic_list":
		actions := make(map[string]bool, len(node.Items))
		for _, item := range node.Items {
			actions[item.Action] = true
		}
		return addComponentPolicy(policy, node.ID, node.Component, actions)
	case "input", "textarea":
		return addComponentPolicy(policy, node.ID, node.Component, nil)
	}
	return nil
}

func addComponentPolicy(policy *renderEventPolicy, id, component string, actions map[string]bool) error {
	if id == "" {
		return fmt.Errorf("interactive component %q is missing an id", component)
	}
	if _, exists := policy.components[id]; exists {
		return fmt.Errorf("duplicate interactive component id %q in render tree", id)
	}
	policy.components[id] = componentEventPolicy{
		component: component,
		actions:   actions,
	}
	return nil
}

func validateEventAgainstPolicy(event protocol.UIEvent, policy renderEventPolicy) error {
	switch event.Kind {
	case protocol.EventKindAction:
		component, err := policy.component(event.Target)
		if err != nil {
			return err
		}
		switch component.component {
		case "button", "checkbox", "menu", "dynamic_list":
		default:
			return fmt.Errorf("action events are not allowed for %q", component.component)
		}
		if !component.actions[event.Action] {
			return fmt.Errorf("action %q is not available for target %q", event.Action, event.Target)
		}
	case protocol.EventKindSelect:
		component, err := policy.component(event.Target)
		if err != nil {
			return err
		}
		if component.component != "list" {
			return fmt.Errorf("select events are not allowed for %q", component.component)
		}
		if !component.actions[event.Action] {
			return fmt.Errorf("selection %q is not available for target %q", event.Action, event.Target)
		}
	case protocol.EventKindSubmit:
		component, err := policy.component(event.Target)
		if err != nil {
			return err
		}
		switch component.component {
		case "input", "textarea":
		default:
			return fmt.Errorf("submit events are not allowed for %q", component.component)
		}
		if len(event.Values) != 1 {
			return fmt.Errorf("submit events for %q must include exactly one field value", event.Target)
		}
		if _, ok := event.Values[event.Target]; !ok {
			return fmt.Errorf("submit events for %q must include a matching field value", event.Target)
		}
	case protocol.EventKindKey:
		if !policy.captureKeys {
			return fmt.Errorf("key events require capture_keys = true in the active render")
		}
		if event.Key == "" {
			return fmt.Errorf("key events require a key value")
		}
	case protocol.EventKindFocus:
		if _, err := policy.component(event.Target); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported event kind %q", event.Kind)
	}
	return nil
}

func (p renderEventPolicy) component(target string) (componentEventPolicy, error) {
	if target == "" {
		return componentEventPolicy{}, fmt.Errorf("event target is required")
	}
	component, ok := p.components[target]
	if !ok {
		return componentEventPolicy{}, fmt.Errorf("event target %q is not present in the active render", target)
	}
	return component, nil
}
