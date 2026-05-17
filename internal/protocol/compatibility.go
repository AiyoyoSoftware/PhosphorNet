package protocol

import (
	"fmt"
	"slices"
)

const ClientVersion = "dev"

func DefaultRenderLimits() RenderLimits {
	return RenderLimits{
		MaxWebSocketMessageBytes:   MaxWebSocketMessageBytes,
		MaxUINodeDepth:             MaxUINodeDepth,
		MaxUIChildren:              MaxUIChildren,
		MaxUIItems:                 MaxUIItems,
		MaxUIGradientStops:         MaxUIGradientStops,
		MaxGridRows:                MaxGridRows,
		MaxGridCols:                MaxGridCols,
		MaxSingleLineTextRunes:     MaxSingleLineTextRunes,
		MaxMultilineTextRunes:      MaxMultilineTextRunes,
		MaxChromeTextRunes:         MaxChromeTextRunes,
		MaxRenderMessagesPerSecond: MaxRenderMessagesPerSecond,
		MaxNotificationsPerMinute:  MaxNotificationsPerMinute,
	}
}

func SupportedUIComponents() []string {
	return []string{
		"screen",
		"header",
		"status",
		"text",
		"markdown",
		"panel",
		"menu",
		"list",
		"dynamic_list",
		"input",
		"textarea",
		"button",
		"checkbox",
		"log",
		"grid",
	}
}

func SupportedStyleFeatures() []string {
	return []string{
		"background:solid",
		"background:gradient",
	}
}

func SupportedEventKinds() []EventKind {
	return []EventKind{
		EventKindAction,
		EventKindSelect,
		EventKindSubmit,
		EventKindKey,
		EventKindFocus,
	}
}

func DefaultHello(publicKey string) HelloMessage {
	return HelloMessage{
		Type:                   TypeHello,
		ClientPublicKey:        publicKey,
		ClientVersion:          ClientVersion,
		RuntimeProtocolVersion: RuntimeContractVersion,
		JSONUISchemaVersion:    JSONUIContractVersion,
		SupportedComponents:    SupportedUIComponents(),
		SupportedStyleFeatures: SupportedStyleFeatures(),
		SupportedEventKinds:    SupportedEventKinds(),
		RenderLimits:           DefaultRenderLimits(),
	}
}

func ValidateClientHello(hello HelloMessage) error {
	if hello.Type != TypeHello {
		return NewCodedError(ErrorProtocol, "expected hello", nil)
	}
	if hello.ClientPublicKey == "" {
		return NewCodedError(ErrorAuth, "hello missing client public key", nil)
	}
	if hello.ClientVersion == "" {
		return NewCodedError(ErrorClientIncompatible, "client hello missing client version", nil)
	}
	if hello.RuntimeProtocolVersion != RuntimeContractVersion {
		return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client understands runtime protocol %q, node requires %q", hello.RuntimeProtocolVersion, RuntimeContractVersion), nil)
	}
	if hello.JSONUISchemaVersion != JSONUIContractVersion {
		return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client understands JSON UI schema %q, node requires %q", hello.JSONUISchemaVersion, JSONUIContractVersion), nil)
	}
	if missing := missingStrings(SupportedUIComponents(), hello.SupportedComponents); len(missing) > 0 {
		return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client is missing supported UI components: %v", missing), nil)
	}
	if missing := missingStrings(SupportedStyleFeatures(), hello.SupportedStyleFeatures); len(missing) > 0 {
		return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client is missing supported style features: %v", missing), nil)
	}
	if missing := missingEventKinds(SupportedEventKinds(), hello.SupportedEventKinds); len(missing) > 0 {
		return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client is missing supported event kinds: %v", missing), nil)
	}
	if err := validateRenderLimits(hello.RenderLimits); err != nil {
		return err
	}
	return nil
}

func missingStrings(required, supported []string) []string {
	missing := []string{}
	for _, value := range required {
		if !slices.Contains(supported, value) {
			missing = append(missing, value)
		}
	}
	return missing
}

func missingEventKinds(required, supported []EventKind) []EventKind {
	missing := []EventKind{}
	for _, value := range required {
		if !slices.Contains(supported, value) {
			missing = append(missing, value)
		}
	}
	return missing
}

func validateRenderLimits(limits RenderLimits) error {
	required := DefaultRenderLimits()
	checks := []struct {
		name string
		got  int
		want int
	}{
		{name: "max_websocket_message_bytes", got: limits.MaxWebSocketMessageBytes, want: required.MaxWebSocketMessageBytes},
		{name: "max_ui_node_depth", got: limits.MaxUINodeDepth, want: required.MaxUINodeDepth},
		{name: "max_ui_children", got: limits.MaxUIChildren, want: required.MaxUIChildren},
		{name: "max_ui_items", got: limits.MaxUIItems, want: required.MaxUIItems},
		{name: "max_ui_gradient_stops", got: limits.MaxUIGradientStops, want: required.MaxUIGradientStops},
		{name: "max_grid_rows", got: limits.MaxGridRows, want: required.MaxGridRows},
		{name: "max_grid_cols", got: limits.MaxGridCols, want: required.MaxGridCols},
		{name: "max_single_line_text_runes", got: limits.MaxSingleLineTextRunes, want: required.MaxSingleLineTextRunes},
		{name: "max_multiline_text_runes", got: limits.MaxMultilineTextRunes, want: required.MaxMultilineTextRunes},
		{name: "max_chrome_text_runes", got: limits.MaxChromeTextRunes, want: required.MaxChromeTextRunes},
		{name: "max_render_messages_per_second", got: limits.MaxRenderMessagesPerSecond, want: required.MaxRenderMessagesPerSecond},
		{name: "max_notifications_per_minute", got: limits.MaxNotificationsPerMinute, want: required.MaxNotificationsPerMinute},
	}
	for _, check := range checks {
		if check.got < check.want {
			return NewCodedError(ErrorClientIncompatible, fmt.Sprintf("client render limit %s=%d is below node requirement %d", check.name, check.got, check.want), nil)
		}
	}
	return nil
}
