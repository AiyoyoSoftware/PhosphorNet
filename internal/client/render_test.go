package client

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"phosphornet/internal/protocol"
)

func TestRenderComponentsDiscoversAllFocusableComponents(t *testing.T) {
	view := protocol.Screen(
		protocol.Panel("Compose",
			protocol.Input("message", "type here", ""),
			protocol.Button("send", "Send", "send_message"),
		),
		protocol.Menu("actions",
			protocol.Item{Label: "Ping", Action: "ping"},
			protocol.Item{Label: "Clear", Action: "clear"},
		),
	)

	rendered, state := RenderComponents(view, renderOptions{
		Width:         80,
		FocusedIndex:  0,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if !strings.Contains(rendered, "Compose") {
		t.Fatal("rendered view does not include panel title")
	}
	if len(state.focusables) != 3 {
		t.Fatalf("len(state.focusables) = %d, want 3", len(state.focusables))
	}
	if state.focusables[0].ID != "message" || state.focusables[1].ID != "send" || state.focusables[2].ID != "actions" {
		t.Fatalf("focusable order = %#v, want message/send/actions", state.focusables)
	}
}

func TestRenderDynamicListCreatesFocusableRows(t *testing.T) {
	view := protocol.UINode{
		Component: "dynamic_list",
		ID:        "door-order-list",
		Items: []protocol.Item{
			{Label: "01. Lobby", Action: "select_door_order:lobby"},
			{Label: "02. Chat", Action: "select_door_order:chat"},
			{Label: "03. Forum", Action: "select_door_order:forum"},
		},
	}

	rendered, state := RenderComponents(view, renderOptions{
		Width:         80,
		FocusedIndex:  1,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 3 {
		t.Fatalf("len(state.focusables) = %d, want 3", len(state.focusables))
	}
	if state.focusables[0].ID != "door-order-list" || state.focusables[1].ID != "door-order-list" || state.focusables[2].ID != "door-order-list" {
		t.Fatalf("focusables = %#v, want door-order-list rows", state.focusables)
	}
	if !strings.Contains(rendered, "> 02. Chat") {
		t.Fatalf("rendered dynamic list = %q, want focused second row", rendered)
	}
}

func TestRemoteEventForFocusedComponents(t *testing.T) {
	model := tuiModel{
		focusedRemote: 0,
		remoteState: renderState{focusables: []focusableComponent{
			{Kind: focusableInput, ID: "message"},
			{Kind: focusableButton, ID: "send", Action: "send_message"},
			{Kind: focusableCheckbox, ID: "door-enabled-chat", Action: "toggle_door_enabled", Checked: true},
			{Kind: focusableMenu, ID: "actions", Items: []protocol.Item{{Label: "Ping", Action: "ping"}}, ItemCount: 1},
		}},
		itemSelection: map[string]int{},
		inputValues:   map[string]string{"message": "hello"},
	}

	event, ok := model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() ok = false")
	}
	if event.Kind != protocol.EventKindSubmit || event.Target != "message" || event.Values["message"] != "hello" {
		t.Fatalf("input event = %#v, want submit with message value", event)
	}

	model.focusedRemote = 1
	event, ok = model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() button ok = false")
	}
	if event.Kind != protocol.EventKindAction || event.Target != "send" || event.Action != "send_message" {
		t.Fatalf("button event = %#v, want send action", event)
	}
	if event.Values["message"] != "hello" {
		t.Fatalf("button event values = %#v, want current input values", event.Values)
	}

	model.focusedRemote = 2
	event, ok = model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() checkbox ok = false")
	}
	if event.Kind != protocol.EventKindAction || event.Target != "door-enabled-chat" || event.Action != "toggle_door_enabled" {
		t.Fatalf("checkbox event = %#v, want toggle action", event)
	}
	if event.Values["checked"] != "false" {
		t.Fatalf("checkbox event values = %#v, want checked=false", event.Values)
	}

	model.focusedRemote = 3
	event, ok = model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() menu ok = false")
	}
	if event.Kind != protocol.EventKindAction || event.Target != "actions" || event.Action != "ping" {
		t.Fatalf("menu event = %#v, want ping action", event)
	}
}

func TestRenderCheckboxUsesSemanticCheckmark(t *testing.T) {
	rendered, state := RenderComponents(protocol.Checkbox("door-enabled-chat", "chat - Chat", true, "toggle_door_enabled"), renderOptions{
		Width:         80,
		FocusedIndex:  0,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 1 {
		t.Fatalf("len(state.focusables) = %d, want 1", len(state.focusables))
	}
	if state.focusables[0].Kind != focusableCheckbox || !state.focusables[0].Checked {
		t.Fatalf("focusable = %#v, want checked checkbox", state.focusables[0])
	}
	if !strings.Contains(rendered, "[x] chat - Chat") {
		t.Fatalf("rendered checkbox = %q, want checked label", rendered)
	}
}

func TestRenderInputUsesSingleStableLine(t *testing.T) {
	rendered, state := RenderComponents(protocol.Input("station-notice", "message to notify all connected users", ""), renderOptions{
		Width:         80,
		FocusedIndex:  0,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 1 {
		t.Fatalf("len(state.focusables) = %d, want 1", len(state.focusables))
	}
	if strings.Contains(rendered, "\n") {
		t.Fatalf("rendered input contains newline: %q", rendered)
	}
	if !strings.Contains(rendered, "message to notify all connected users") {
		t.Fatalf("rendered input = %q, want placeholder text", rendered)
	}
}

func TestRenderFocusedTextareaStaysSingleLineAtExactWidth(t *testing.T) {
	placeholder := strings.Repeat("x", 16)
	rendered, state := RenderComponents(protocol.Textarea("bio", placeholder, ""), renderOptions{
		Width:         20,
		FocusedIndex:  0,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 1 {
		t.Fatalf("len(state.focusables) = %d, want 1", len(state.focusables))
	}
	if strings.Contains(rendered, "\n") {
		t.Fatalf("rendered textarea contains newline at exact width: %q", rendered)
	}
}

func TestRenderPanelInputFitsInsideContainerWidth(t *testing.T) {
	placeholder := "/msg #station"
	rendered, state := RenderComponents(protocol.Panel("Setting",
		protocol.Input("setting-value", placeholder, ""),
	), renderOptions{
		Width:         80,
		FocusedIndex:  0,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 1 {
		t.Fatalf("len(state.focusables) = %d, want 1", len(state.focusables))
	}
	plain := stripANSISequences(rendered)
	if width := maxRenderedLineWidth(plain); width > 80 {
		t.Fatalf("rendered panel width = %d, want <= 80:\n%s", width, plain)
	}
	if !strings.Contains(plain, "> "+placeholder) {
		t.Fatalf("rendered panel input did not keep prefix and placeholder on one line:\n%s", plain)
	}
}

func TestRenderPanelSupportsGradientBackgroundStyle(t *testing.T) {
	view := protocol.Panel("Station",
		protocol.Text("PHOSPHOR LABS LOG"),
		protocol.Text("Client renders. Station thinks."),
	)
	view.Style = &protocol.UIStyle{Background: &protocol.UIBackground{
		Kind:      "gradient",
		Direction: "vertical",
		From:      "#18122b",
		To:        "#2b124c",
	}}

	rendered, state := RenderComponents(view, renderOptions{
		Width:         80,
		FocusedIndex:  -1,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 0 {
		t.Fatalf("len(state.focusables) = %d, want 0", len(state.focusables))
	}
	plain := stripANSISequences(rendered)
	if !strings.Contains(plain, "Station") || !strings.Contains(plain, "PHOSPHOR LABS LOG") {
		t.Fatalf("rendered gradient panel missing content:\n%s", plain)
	}
	background, ok := resolveContainerBackground(view.Style)
	if !ok {
		t.Fatal("resolveContainerBackground() ok = false")
	}
	if got := gradientColorAt(background.stops, 0.5); got != lipgloss.Color("#22123c") {
		t.Fatalf("gradient midpoint = %q, want #22123c", got)
	}
}

func TestRenderPanelBackgroundStaysInsideRoundedBorder(t *testing.T) {
	view := protocol.Panel("Station", protocol.Text("inside"))
	view.Style = &protocol.UIStyle{Background: &protocol.UIBackground{
		Kind:  "solid",
		Color: "#18122b",
	}}

	rendered, _ := RenderComponents(view, renderOptions{
		Width:         48,
		FocusedIndex:  -1,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		t.Fatalf("rendered panel line count = %d, want at least 3:\n%q", len(lines), rendered)
	}
	background := "\x1b[48;2;24;18;43m"
	if strings.Contains(lines[0], background) {
		t.Fatalf("top border line has panel background, want transparent rounded corners: %q", lines[0])
	}
	if !strings.Contains(lines[1], background) {
		t.Fatalf("interior line missing panel background: %q", lines[1])
	}
}

func TestPaintLineBackgroundSurvivesNestedStyleReset(t *testing.T) {
	line := "\x1b[38;2;255;255;255mPHOSPHOR\x1b[0m LABS"

	painted := paintLineBackground(line, lipgloss.Color("#18122b"))

	if !strings.Contains(painted, "\x1b[0m\x1b[48;2;24;18;43m LABS") {
		t.Fatalf("painted line did not reapply background after nested reset: %q", painted)
	}
}

func TestRenderScreenBackgroundFillsViewportArea(t *testing.T) {
	view := protocol.Screen(protocol.Text("Lobby"))
	view.Style = &protocol.UIStyle{Background: &protocol.UIBackground{
		Kind:  "solid",
		Color: "#0b1020",
	}}

	rendered, _ := RenderComponents(view, renderOptions{
		Width:          24,
		ViewportHeight: 4,
		FocusedIndex:   -1,
		ItemSelection:  map[string]int{},
		InputValues:    map[string]string{},
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("rendered line count = %d, want 4:\n%q", len(lines), rendered)
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width != 24 {
			t.Fatalf("line %d width = %d, want 24: %q", index, width, line)
		}
	}
}

func TestSanitizeRemoteNodeDropsStyleFromLeafComponents(t *testing.T) {
	view := protocol.Button("send", "Send", "send")
	view.Style = &protocol.UIStyle{Background: &protocol.UIBackground{
		Kind:  "solid",
		Color: "#18122b",
	}}

	sanitized := sanitizeRemoteNode(view, 0)

	if sanitized.Style != nil {
		t.Fatalf("sanitized button style = %#v, want nil", sanitized.Style)
	}
}

func TestRenderMarkdownUsesTerminalRenderer(t *testing.T) {
	rendered, state := RenderComponents(protocol.Markdown("# Hello\n\nThis is a *forum* post."), renderOptions{
		Width:         80,
		FocusedIndex:  -1,
		ItemSelection: map[string]int{},
		InputValues:   map[string]string{},
	})

	if len(state.focusables) != 0 {
		t.Fatalf("len(state.focusables) = %d, want 0", len(state.focusables))
	}
	if !strings.Contains(rendered, "Hello") {
		t.Fatalf("rendered markdown = %q, want heading text", rendered)
	}
	if strings.Contains(rendered, "# Hello") {
		t.Fatalf("rendered markdown = %q, want terminal rendering rather than raw markdown", rendered)
	}
}

func TestScrollableTextClipsAndReportsPosition(t *testing.T) {
	value := strings.Join([]string{"one", "two", "three", "four"}, "\n")

	visible, label := scrollableText(value, 1, 2)

	if visible != "two\nthree" {
		t.Fatalf("visible = %q, want %q", visible, "two\nthree")
	}
	if label != "[2-3/4]" {
		t.Fatalf("label = %q, want %q", label, "[2-3/4]")
	}
}

func TestRemoteScrollClampsToRenderedContent(t *testing.T) {
	model := tuiModel{
		width:        100,
		height:       18,
		remoteScroll: 100,
		currentView: protocol.Screen(
			protocol.Text("one"),
			protocol.Text("two"),
			protocol.Text("three"),
			protocol.Text("four"),
			protocol.Text("five"),
			protocol.Text("six"),
			protocol.Text("seven"),
			protocol.Text("eight"),
			protocol.Text("nine"),
			protocol.Text("ten"),
			protocol.Text("eleven"),
			protocol.Text("twelve"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}

	model.clampRemoteScroll()

	if model.remoteScroll > renderedLineCount(model.renderRemoteView())-model.remoteViewportHeight() {
		t.Fatalf("remoteScroll = %d, want clamped to content", model.remoteScroll)
	}
	if model.remoteScroll < 0 {
		t.Fatalf("remoteScroll = %d, want non-negative", model.remoteScroll)
	}
}

func TestRenderMessageScrollsBottomOnOpenForBottomPinnedView(t *testing.T) {
	model := tuiModel{
		width:         100,
		height:        14,
		openingDoor:   "chat",
		currentView:   protocol.Screen(protocol.Text("old")),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	view := tallScreen("line", 20)
	view.Scroll = "bottom"

	updated, _ := model.Update(protocol.RenderMessage{SessionID: "s1", View: view})
	got := updated.(tuiModel)

	if got.remoteScroll != got.maxRemoteScroll() {
		t.Fatalf("remoteScroll = %d, want bottom %d", got.remoteScroll, got.maxRemoteScroll())
	}
}

func TestRenderMessageKeepsBottomPinnedViewAtBottom(t *testing.T) {
	oldView := tallScreen("old", 20)
	oldView.Scroll = "bottom"
	model := tuiModel{
		width:         100,
		height:        14,
		currentView:   oldView,
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.scrollRemoteToBottom()
	newView := tallScreen("new", 30)
	newView.Scroll = "bottom"

	updated, _ := model.Update(protocol.RenderMessage{SessionID: "s1", View: newView})
	got := updated.(tuiModel)

	if got.remoteScroll != got.maxRemoteScroll() {
		t.Fatalf("remoteScroll = %d, want bottom %d", got.remoteScroll, got.maxRemoteScroll())
	}
}

func TestRenderMessageDoesNotForceBottomWhenUserScrolledUp(t *testing.T) {
	oldView := tallScreen("old", 20)
	oldView.Scroll = "bottom"
	model := tuiModel{
		width:         100,
		height:        14,
		remoteScroll:  0,
		currentView:   oldView,
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	newView := tallScreen("new", 30)
	newView.Scroll = "bottom"

	updated, _ := model.Update(protocol.RenderMessage{SessionID: "s1", View: newView})
	got := updated.(tuiModel)

	if got.remoteScroll != 0 {
		t.Fatalf("remoteScroll = %d, want preserved scroll-up position 0", got.remoteScroll)
	}
}

func TestBottomPinnedInputRendersAtViewportBottomWhenContentIsShort(t *testing.T) {
	input := protocol.Input("chat-message", "/msg #station", "")
	input.Dock = "bottom"
	view := protocol.Screen(
		protocol.Header("#station"),
		protocol.Text("topic: test"),
		protocol.Text("<user> hello"),
		input,
	)
	view.Scroll = "bottom"

	rendered, state := RenderComponents(view, renderOptions{
		Width:          80,
		ViewportHeight: 12,
		FocusedIndex:   0,
		ItemSelection:  map[string]int{},
		InputValues:    map[string]string{},
	})
	lines := splitRenderableLines(rendered)

	if len(state.focusables) != 1 || state.focusables[0].ID != "chat-message" {
		t.Fatalf("focusables = %#v, want chat-message input", state.focusables)
	}
	if len(lines) != 12 {
		t.Fatalf("rendered line count = %d, want 12\n%s", len(lines), rendered)
	}
	if !strings.Contains(lines[len(lines)-1], "/msg #station") {
		t.Fatalf("last rendered line = %q, want chat input at bottom", lines[len(lines)-1])
	}
}

func TestBottomPinnedScreenRequiresDoorDockHint(t *testing.T) {
	view := protocol.Screen(
		protocol.Text("topic: test"),
		protocol.Input("chat-message", "/msg #station", ""),
	)
	view.Scroll = "bottom"

	rendered, _ := RenderComponents(view, renderOptions{
		Width:          80,
		ViewportHeight: 8,
		FocusedIndex:   0,
		ItemSelection:  map[string]int{},
		InputValues:    map[string]string{},
	})

	if len(splitRenderableLines(rendered)) >= 8 {
		t.Fatalf("rendered line count = %d, want no implicit bottom dock\n%s", len(splitRenderableLines(rendered)), rendered)
	}
}

func TestFocusedRemoteInputConsumesLetterShortcuts(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 0,
		currentView:   protocol.Screen(protocol.Input("chat-message", "/msg #station", "")),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got := updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(f) returned command, want nil")
	}
	if got.fullScreen {
		t.Fatal("fullScreen = true, want focused input to consume f")
	}
	if got.inputValues["chat-message"] != "f" {
		t.Fatalf("input value = %q, want f", got.inputValues["chat-message"])
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(q) returned command, want nil")
	}
	if got.inputValues["chat-message"] != "fq" {
		t.Fatalf("input value = %q, want fq", got.inputValues["chat-message"])
	}
}

func TestEnterOnRemoteInputMovesFocusWithoutSubmitting(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 0,
		currentView: protocol.Screen(
			protocol.Input("profile-display-name", "display name", ""),
			protocol.Input("profile-status-line", "status line", ""),
			protocol.Button("save-profile", "Save Profile", "save_profile"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(enter) on input returned command, want local focus move")
	}
	if got.focusedRemote != 1 {
		t.Fatalf("focusedRemote = %d, want next input index 1", got.focusedRemote)
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(enter) on second input returned command, want local focus move")
	}
	if got.focusedRemote != 2 {
		t.Fatalf("focusedRemote = %d, want save button index 2", got.focusedRemote)
	}
}

func TestEnterOnRemoteTextareaAddsNewline(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 0,
		currentView: protocol.Screen(
			protocol.Textarea("forum-reply-body", "Write your reply in markdown", ""),
			protocol.Button("forum-post-reply", "Post Reply", "post_reply"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{"forum-reply-body": "first line"},
	}
	model.refreshRemoteState()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(enter) on textarea returned command, want local newline edit")
	}
	if got.focusedRemote != 0 {
		t.Fatalf("focusedRemote = %d, want textarea to stay focused", got.focusedRemote)
	}
	if got.inputValues["forum-reply-body"] != "first line\n" {
		t.Fatalf("textarea value = %q, want newline appended", got.inputValues["forum-reply-body"])
	}
}

func TestEnterOnBottomDockedRemoteInputSubmits(t *testing.T) {
	composer := protocol.Input("chat-message", "/msg #station", "")
	composer.Dock = "bottom"
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 0,
		sessionID:     "session-1",
		currentView:   protocol.Screen(composer),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{"chat-message": "hello station"},
	}
	model.refreshRemoteState()

	if len(model.remoteState.focusables) != 1 {
		t.Fatalf("len(focusables) = %d, want 1", len(model.remoteState.focusables))
	}
	if model.remoteState.focusables[0].Dock != "bottom" {
		t.Fatalf("focusable dock = %q, want bottom", model.remoteState.focusables[0].Dock)
	}

	event, ok := model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() ok = false")
	}
	if event.Kind != protocol.EventKindSubmit || event.Target != "chat-message" || event.Values["chat-message"] != "hello station" {
		t.Fatalf("submit event = %#v, want chat-message value", event)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(tuiModel)
	if cmd == nil {
		t.Fatal("Update(enter) on bottom-docked input returned nil command, want submit event send")
	}
	if got.focusedRemote != 0 {
		t.Fatalf("focusedRemote = %d, want to remain on chat input", got.focusedRemote)
	}
	if value := got.inputValues["chat-message"]; value != "" {
		t.Fatalf("chat-message input value = %q, want cleared after submit", value)
	}
	if got.status != "Sending submit event to chat-message..." {
		t.Fatalf("status = %q, want submit send status", got.status)
	}
}

func TestEnterOnRemoteButtonActivatesWithInputValues(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 1,
		sessionID:     "session-1",
		currentView: protocol.Screen(
			protocol.Input("profile-display-name", "display name", ""),
			protocol.Button("save-profile", "Save Profile", "save_profile"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{"profile-display-name": "Ada"},
	}
	model.refreshRemoteState()

	event, ok := model.remoteEventForFocus()
	if !ok {
		t.Fatal("remoteEventForFocus() ok = false")
	}
	if event.Kind != protocol.EventKindAction || event.Target != "save-profile" || event.Action != "save_profile" {
		t.Fatalf("button event = %#v, want save_profile action", event)
	}
	if event.Values["profile-display-name"] != "Ada" {
		t.Fatalf("button values = %#v, want edited profile display name", event.Values)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(tuiModel)
	if cmd == nil {
		t.Fatal("Update(enter) on button returned nil command, want event send")
	}
	if got.status != "Sending action event to save-profile..." {
		t.Fatalf("status = %q, want action send status", got.status)
	}
}

func TestCaptureKeysSendsRawDoorKeyEventsWhenEnabled(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		currentView:   protocol.UINode{Component: "screen", CaptureKeys: true},
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	got := updated.(tuiModel)
	if cmd == nil {
		t.Fatal("Update(+) returned nil command, want key event send")
	}
	if got.status != "Sending key event..." {
		t.Fatalf("status = %q, want key event send status", got.status)
	}
}

func TestCapturedKeyValueUsesTypedRuneForShiftedSymbols(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	if got := capturedKeyValue(msg); got != "+" {
		t.Fatalf("capturedKeyValue(+) = %q, want +", got)
	}
}

func TestCaptureKeysDoesNotStealFocusedTextInputTyping(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		focusedRemote: 0,
		currentView:   protocol.UINode{Component: "screen", CaptureKeys: true, Children: []protocol.UINode{protocol.Input("chat-message", "", "")}},
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	got := updated.(tuiModel)
	if cmd != nil {
		t.Fatal("Update(+) returned command, want focused input to keep local typing")
	}
	if got.inputValues["chat-message"] != "+" {
		t.Fatalf("input value = %q, want +", got.inputValues["chat-message"])
	}
}

func tallScreen(prefix string, lines int) protocol.UINode {
	children := make([]protocol.UINode, 0, lines)
	for index := 0; index < lines; index++ {
		children = append(children, protocol.Text(prefix))
	}
	return protocol.Screen(children...)
}

func TestTabFocusesRemoteContentWithoutFocusableComponents(t *testing.T) {
	model := tuiModel{
		focus:         focusDoors,
		currentView:   protocol.Screen(protocol.Text("plain remote content")),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(tuiModel)

	if got.focus != focusRemote {
		t.Fatalf("focus = %v, want focusRemote", got.focus)
	}
}

func TestTabUnfocusesRemoteContent(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		currentView:   protocol.Screen(protocol.Button("ping", "Ping", "ping")),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(tuiModel)

	if got.focus != focusDoors {
		t.Fatalf("focus = %v, want focusDoors", got.focus)
	}
}

func TestArrowKeysScrollRemoteContentWhenNoComponentIsVisible(t *testing.T) {
	model := tuiModel{
		focus:        focusRemote,
		width:        100,
		height:       14,
		remoteScroll: 0,
		currentView: protocol.Screen(
			protocol.Text("one"),
			protocol.Text("two"),
			protocol.Text("three"),
			protocol.Text("four"),
			protocol.Text("five"),
			protocol.Text("six"),
			protocol.Text("seven"),
			protocol.Text("eight"),
			protocol.Text("nine"),
			protocol.Text("ten"),
			protocol.Text("eleven"),
			protocol.Text("twelve"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(tuiModel)

	if got.remoteScroll == 0 {
		t.Fatal("remoteScroll = 0, want down arrow to scroll remote content")
	}
}

func TestLeftRightKeysCycleVisibleRemoteComponents(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		width:         100,
		height:        24,
		focusedRemote: 0,
		currentView: protocol.Screen(
			protocol.Button("one", "One", "one"),
			protocol.Button("two", "Two", "two"),
			protocol.Button("three", "Three", "three"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := updated.(tuiModel)
	if got.focusedRemote != 1 {
		t.Fatalf("focusedRemote = %d, want 1", got.focusedRemote)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got = updated.(tuiModel)
	if got.focusedRemote != 0 {
		t.Fatalf("focusedRemote = %d, want 0", got.focusedRemote)
	}
}

func TestLeftRightKeysCycleDoorsWhenDoorRailFocused(t *testing.T) {
	model := tuiModel{
		focus: focusDoors,
		doors: []protocol.DoorSummary{
			{ID: "lobby", Name: "Lobby"},
			{ID: "chat", Name: "Chat"},
			{ID: "forum", Name: "Forum"},
		},
		selected:    0,
		doorScroll:  0,
		width:       100,
		height:      16,
		inputValues: map[string]string{},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := updated.(tuiModel)
	if got.selected != 1 {
		t.Fatalf("selected = %d, want 1", got.selected)
	}
	if got.focus != focusDoors {
		t.Fatalf("focus = %v, want focusDoors", got.focus)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got = updated.(tuiModel)
	if got.selected != 0 {
		t.Fatalf("selected = %d, want 0", got.selected)
	}
}

func TestUpDownKeysScrollDoorRailWhenFocused(t *testing.T) {
	model := tuiModel{
		focus: focusDoors,
		doors: []protocol.DoorSummary{
			{ID: "d1", Name: "Door 1"},
			{ID: "d2", Name: "Door 2"},
			{ID: "d3", Name: "Door 3"},
			{ID: "d4", Name: "Door 4"},
			{ID: "d5", Name: "Door 5"},
			{ID: "d6", Name: "Door 6"},
			{ID: "d7", Name: "Door 7"},
			{ID: "d8", Name: "Door 8"},
			{ID: "d9", Name: "Door 9"},
			{ID: "d10", Name: "Door 10"},
			{ID: "d11", Name: "Door 11"},
			{ID: "d12", Name: "Door 12"},
		},
		selected:    0,
		doorScroll:  0,
		width:       100,
		height:      16,
		inputValues: map[string]string{},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(tuiModel)
	if got.doorScroll == 0 {
		t.Fatal("doorScroll = 0, want down arrow to scroll the focused door rail")
	}
	if got.selected != 0 {
		t.Fatalf("selected = %d, want unchanged selected door", got.selected)
	}
}

func TestArrowKeysScrollWhenComponentsAreOffscreen(t *testing.T) {
	model := tuiModel{
		focus:        focusRemote,
		width:        100,
		height:       14,
		remoteScroll: 0,
		currentView: protocol.Screen(
			protocol.Text("one"),
			protocol.Text("two"),
			protocol.Text("three"),
			protocol.Text("four"),
			protocol.Text("five"),
			protocol.Text("six"),
			protocol.Text("seven"),
			protocol.Text("eight"),
			protocol.Text("nine"),
			protocol.Text("ten"),
			protocol.Button("late", "Late Button", "late"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(tuiModel)
	if got.focusedRemote != -1 {
		t.Fatalf("focusedRemote = %d, want cleared focus", got.focusedRemote)
	}
	if got.remoteScroll == 0 {
		t.Fatal("remoteScroll = 0, want scroll when no components are visible")
	}
}

func TestArrowKeysScrollBeyondLastVisibleComponent(t *testing.T) {
	model := tuiModel{
		focus:         focusRemote,
		width:         100,
		height:        18,
		focusedRemote: 1,
		remoteScroll:  0,
		currentView: protocol.Screen(
			protocol.Button("one", "One", "one"),
			protocol.Button("two", "Two", "two"),
			protocol.Text("three"),
			protocol.Text("four"),
			protocol.Text("five"),
			protocol.Text("six"),
			protocol.Text("seven"),
			protocol.Text("eight"),
			protocol.Text("nine"),
			protocol.Text("ten"),
			protocol.Text("eleven"),
			protocol.Text("twelve"),
		),
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(tuiModel)
	if got.focusedRemote != -1 {
		t.Fatalf("focusedRemote = %d, want to clear stale component focus", got.focusedRemote)
	}
	if got.remoteScroll == 0 {
		t.Fatal("remoteScroll = 0, want scroll beyond last visible component")
	}

	event, ok := got.remoteEventForFocus()
	if ok {
		t.Fatalf("remoteEventForFocus() = %#v, true; want no event after scrolling away", event)
	}

	previousScroll := got.remoteScroll
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(tuiModel)
	if got.focusedRemote != -1 {
		t.Fatalf("focusedRemote = %d, want focus to stay cleared while same component remains visible", got.focusedRemote)
	}
	if got.remoteScroll <= previousScroll {
		t.Fatalf("remoteScroll = %d, want continued scroll beyond %d", got.remoteScroll, previousScroll)
	}
}

func TestFullscreenUsesWiderRemoteViewport(t *testing.T) {
	model := tuiModel{
		width:  120,
		height: 32,
	}
	stationHeight := model.remoteViewportHeight()
	stationWidth := model.remoteRenderWidth()

	model.fullScreen = true
	fullscreenHeight := model.remoteViewportHeight()
	fullscreenWidth := model.remoteRenderWidth()

	if fullscreenHeight < stationHeight {
		t.Fatalf("fullscreenHeight = %d, want at least station height %d", fullscreenHeight, stationHeight)
	}
	if fullscreenWidth <= stationWidth {
		t.Fatalf("fullscreenWidth = %d, want greater than station width %d", fullscreenWidth, stationWidth)
	}
}

func TestStationViewportUsesAvailableVerticalSpace(t *testing.T) {
	model := tuiModel{
		width:  120,
		height: 32,
	}

	if got := model.remoteViewportHeight(); got != 24 {
		t.Fatalf("remoteViewportHeight() = %d, want 24", got)
	}
}

func TestStationRemoteRenderWidthMatchesViewportInterior(t *testing.T) {
	model := tuiModel{
		width:  120,
		height: 32,
	}

	if got := model.remoteRenderWidth(); got != 82 {
		t.Fatalf("remoteRenderWidth() = %d, want 82", got)
	}
}

func TestStationViewDoesNotRenderRemoteFocusPanel(t *testing.T) {
	model := tuiModel{
		auth:          protocol.AuthOKMessage{NodeName: "localbox", NodeID: "node:test", Fingerprint: "USER-TEST-0000"},
		doors:         []protocol.DoorSummary{{ID: "lobby", Name: "Lobby"}},
		currentView:   protocol.Screen(protocol.Button("ping", "Ping", "ping")),
		width:         120,
		height:        32,
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}
	model.refreshRemoteState()

	rendered := model.View()
	if strings.Contains(rendered, "Remote Focus") {
		t.Fatal("station view renders Remote Focus panel")
	}
	if !strings.Contains(rendered, "Doors") {
		t.Fatal("station view does not render door menu")
	}
	if strings.Contains(rendered, "Remote View") {
		t.Fatal("station view renders removed remote view header")
	}
	if !strings.Contains(rendered, "Ping") {
		t.Fatal("station view does not render remote content")
	}
}

func TestStationViewHeaderUsesTwoLines(t *testing.T) {
	model := tuiModel{
		auth:          protocol.AuthOKMessage{NodeName: "localbox", NodeID: "node:test", Fingerprint: "USER-TEST-0000"},
		currentView:   protocol.Screen(protocol.Text("hello")),
		width:         120,
		height:        32,
		itemSelection: map[string]int{},
		inputValues:   map[string]string{},
	}

	rendered := model.View()
	if strings.Contains(rendered, "\nNode ID:") || strings.Contains(rendered, "\nYou:") {
		t.Fatalf("header metadata rendered on separate lines:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Node: localbox") || !strings.Contains(rendered, "Node ID: node:test") || !strings.Contains(rendered, "You: USER-TEST-0000") {
		t.Fatalf("header missing metadata:\n%s", rendered)
	}
}

func TestFilterVisibleDoorsHidesPrivateAndHiddenDoors(t *testing.T) {
	doors := []protocol.DoorSummary{
		{ID: "lobby", Name: "Lobby"},
		{ID: "members", Name: "Members", Visibility: "private"},
		{ID: "ops", Name: "Ops", Visibility: "hidden"},
		{ID: "chat", Name: "Chat", Visibility: "public"},
	}

	visible := filterVisibleDoors(doors)

	if len(visible) != 2 {
		t.Fatalf("len(visible) = %d, want 2", len(visible))
	}
	if visible[0].ID != "lobby" || visible[1].ID != "chat" {
		t.Fatalf("visible doors = %#v, want lobby/chat", visible)
	}
}

func TestRenderComponentsSanitizesHostileRemoteStrings(t *testing.T) {
	view := protocol.Screen(
		protocol.Header("Node\x1b[31m"),
		protocol.Text("alpha\u202eomega"),
		protocol.Text(strings.Repeat("😀", 512)),
		protocol.Panel("panel\u200btitle",
			protocol.Text("line 1\r\nline 2\t\u2066spoof\u2069"),
			protocol.Button("send", "Send\x1b[0m", "send\x1b[0m"),
		),
		protocol.Menu("menu", protocol.Item{Label: "Item\u200dOne", Action: "do"}),
		protocol.Grid("grid", [][]string{{"A\x1b[2J", "B"}, {"C", "D"}}),
	)

	rendered, _ := RenderComponents(view, renderOptions{Width: 80, FocusedIndex: -1, ItemSelection: map[string]int{}, InputValues: map[string]string{}})
	plain := stripANSISequences(rendered)

	for _, dangerous := range []rune{'\x1b', '\u202e', '\u200b', '\u200c', '\u200d', '\u2066', '\u2067', '\u2068', '\u2069'} {
		if strings.ContainsRune(plain, dangerous) {
			t.Fatalf("rendered output contains dangerous rune %U:\n%s", dangerous, rendered)
		}
	}
	if !strings.Contains(plain, "Node") || !strings.Contains(plain, "omega") {
		t.Fatalf("rendered output missing sanitized remote text:\n%s", rendered)
	}
}

func TestRenderComponentsHandlesUnknownAndDeepTrees(t *testing.T) {
	leaf := protocol.Text("deep leaf")
	for i := 0; i < protocol.MaxUINodeDepth+8; i++ {
		leaf = protocol.Panel("layer", leaf)
	}

	wideChildren := make([]protocol.UINode, 0, protocol.MaxUIChildren+25)
	for i := 0; i < protocol.MaxUIChildren+25; i++ {
		wideChildren = append(wideChildren, protocol.Text("child"))
	}

	view := protocol.Screen(
		leaf,
		protocol.UINode{Component: "mystery", Children: []protocol.UINode{protocol.Text("should not render")}},
		protocol.Screen(wideChildren...),
	)

	rendered, _ := RenderComponents(view, renderOptions{Width: 80, FocusedIndex: -1, ItemSelection: map[string]int{}, InputValues: map[string]string{}})

	if !strings.Contains(rendered, "[render") || !strings.Contains(rendered, "exceeded]") {
		t.Fatalf("rendered output missing depth warning:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[unsupported component]") {
		t.Fatalf("rendered output missing unknown-component warning:\n%s", rendered)
	}
	if strings.Contains(rendered, "should not render") {
		t.Fatalf("rendered output leaked children of unknown component:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[content truncated]") {
		t.Fatalf("rendered output missing truncation warning:\n%s", rendered)
	}
}

func TestTUIRateLimitCounters(t *testing.T) {
	model := tuiModel{}
	now := time.Now()

	for i := 0; i < protocol.MaxRenderMessagesPerSecond; i++ {
		if !model.allowRender(now) {
			t.Fatalf("allowRender() denied render %d within limit", i+1)
		}
	}
	if model.allowRender(now) {
		t.Fatal("allowRender() allowed render above per-second limit")
	}
	if !model.allowRender(now.Add(time.Second)) {
		t.Fatal("allowRender() did not reset after a second")
	}

	for i := 0; i < protocol.MaxNotificationsPerMinute; i++ {
		if !model.allowNotification(now) {
			t.Fatalf("allowNotification() denied notification %d within limit", i+1)
		}
	}
	if model.allowNotification(now) {
		t.Fatal("allowNotification() allowed notification above per-minute limit")
	}
	if !model.allowNotification(now.Add(time.Minute)) {
		t.Fatal("allowNotification() did not reset after a minute")
	}
}

func stripANSISequences(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '\x1b' {
			out.WriteByte(value[i])
			i++
			continue
		}
		if i+1 >= len(value) {
			break
		}
		switch value[i+1] {
		case '[':
			i += 2
			for i < len(value) {
				c := value[i]
				if c >= 0x40 && c <= 0x7e {
					i++
					break
				}
				i++
			}
		case ']':
			i += 2
			for i < len(value) {
				if value[i] == '\a' {
					i++
					break
				}
				if value[i] == '\x1b' && i+1 < len(value) && value[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			i += 2
		}
	}
	return out.String()
}

func maxRenderedLineWidth(value string) int {
	width := 0
	for _, line := range strings.Split(value, "\n") {
		if lineWidth := len([]rune(line)); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}
