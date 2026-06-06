package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/protocol"
)

type errMsg struct {
	err error
}

type connectionClosedMsg struct {
	reason string
}

type doorOpenedMsg struct {
	doorID string
}

type eventSentMsg struct{}

type focusArea int

const (
	focusDoors focusArea = iota
	focusRemote
)

type tuiModel struct {
	conn              *websocket.Conn
	auth              protocol.AuthOKMessage
	sessionID         string
	activeDoorID      string
	renderRevision    int64
	doors             []protocol.DoorSummary
	selected          int
	doorScroll        int
	currentView       protocol.UINode
	status            string
	lastError         string
	openingDoor       string
	fullScreen        bool
	focus             focusArea
	focusedRemote     int
	remoteScroll      int
	remoteState       renderState
	itemSelection     map[string]int
	inputValues       map[string]string
	width             int
	height            int
	renderWindowStart time.Time
	renderWindowCount int
	notifyWindowStart time.Time
	notifyWindowCount int
}

func newTUIModel(conn *websocket.Conn, auth protocol.AuthOKMessage, doors []protocol.DoorSummary, initialRender protocol.RenderMessage, status string) tuiModel {
	visibleDoors := filterVisibleDoors(doors)
	itemSelection := map[string]int{}
	inputValues := map[string]string{}
	_, state := RenderComponents(initialRender.View, renderOptions{
		Width:         80,
		FocusedIndex:  -1,
		ItemSelection: itemSelection,
		InputValues:   inputValues,
	})
	return tuiModel{
		conn:           conn,
		auth:           auth,
		sessionID:      initialRender.SessionID,
		activeDoorID:   initialRender.ActiveDoorID,
		renderRevision: initialRender.RenderRevision,
		doors:          visibleDoors,
		selected:       preferredDoorIndex(visibleDoors, "lobby"),
		currentView:    initialRender.View,
		status:         status,
		focus:          focusDoors,
		focusedRemote:  0,
		remoteState:    state,
		itemSelection:  itemSelection,
		inputValues:    inputValues,
		width:          120,
		height:         32,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		capturedKey := capturedKeyValue(msg)
		if m.focus == focusRemote && m.focusedComponentIsTextInput() && len(msg.Runes) > 0 {
			m.appendRemoteInput(string(msg.Runes))
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "f":
			m.fullScreen = !m.fullScreen
			if m.fullScreen {
				m.focus = focusRemote
			}
			m.clampRemoteScroll()
		case "tab":
			if m.focus == focusDoors {
				m.focus = focusRemote
			} else {
				m.focus = focusDoors
			}
		case "shift+tab":
			if m.focus == focusRemote {
				m.focus = focusDoors
			} else {
				m.focus = focusRemote
			}
		case "right":
			if m.focus == focusDoors {
				m.moveDoorSelection(1)
			} else if m.focus == focusRemote {
				m.moveRemoteFocus(1)
			}
		case "left":
			if m.focus == focusDoors {
				m.moveDoorSelection(-1)
			} else if m.focus == focusRemote {
				m.moveRemoteFocus(-1)
			}
		case "esc":
			if m.fullScreen {
				m.fullScreen = false
				m.clampRemoteScroll()
				return m, nil
			}
			m.focus = focusDoors
		case "pgup", "ctrl+u":
			if m.scrollRemote(-m.remotePageSize()) {
				m.clearRemoteFocus()
			}
		case "pgdown", "ctrl+d":
			if m.scrollRemote(m.remotePageSize()) {
				m.clearRemoteFocus()
			}
		case "home":
			previous := m.remoteScroll
			m.remoteScroll = 0
			if m.remoteScroll != previous {
				m.clearRemoteFocus()
			}
		case "end":
			previous := m.remoteScroll
			m.remoteScroll = maxInt(renderedLineCount(m.renderRemoteView())-m.remoteViewportHeight(), 0)
			if m.remoteScroll != previous {
				m.clearRemoteFocus()
			}
		case "up":
			if m.focus == focusRemote {
				if m.scrollRemote(-1) {
					m.clearRemoteFocus()
				}
			} else {
				m.scrollDoors(-1)
			}
		case "down":
			if m.focus == focusRemote {
				if m.scrollRemote(1) {
					m.clearRemoteFocus()
				}
			} else {
				m.scrollDoors(1)
			}
		case "enter":
			if m.focus == focusRemote {
				if m.focusedComponentIsTextarea() {
					m.appendRemoteInput("\n")
					return m, nil
				}
				if m.focusedComponentIsTextInput() && !m.focusedComponentSubmitsOnEnter() {
					m.moveRemoteFocusCycle(1)
					return m, nil
				}
				event, ok := m.remoteEventForFocus()
				if !ok {
					return m, nil
				}
				if m.focusedComponentSubmitsOnEnter() {
					delete(m.inputValues, event.Target)
				}
				m.status = fmt.Sprintf("Sending %s event to %s...", sanitizeChromeText(string(event.Kind)), sanitizeChromeText(event.Target))
				return m, sendEventCmd(m.conn, m.sessionID, m.activeDoorID, m.renderRevision, event)
			}
			if len(m.doors) == 0 {
				return m, nil
			}
			door := m.doors[m.selected]
			m.openingDoor = door.ID
			m.status = fmt.Sprintf("Opening %s...", sanitizeChromeText(door.Name))
			return m, openDoorCmd(m.conn, door.ID)
		case " ":
			if m.focus == focusRemote {
				component, ok := m.focusedComponent()
				if ok && (component.Kind == focusableInput || component.Kind == focusableTextarea) {
					m.appendRemoteInput(" ")
					return m, nil
				}
				event, ok := m.remoteEventForFocus()
				if !ok {
					return m, nil
				}
				m.status = fmt.Sprintf("Sending %s event to %s...", sanitizeChromeText(string(event.Kind)), sanitizeChromeText(event.Target))
				return m, sendEventCmd(m.conn, m.sessionID, m.activeDoorID, m.renderRevision, event)
			}
			if len(m.doors) == 0 {
				return m, nil
			}
			door := m.doors[m.selected]
			m.openingDoor = door.ID
			m.status = fmt.Sprintf("Opening %s...", sanitizeChromeText(door.Name))
			return m, openDoorCmd(m.conn, door.ID)
		case "backspace", "ctrl+h":
			if m.focus == focusRemote {
				m.backspaceRemoteInput()
			} else if m.shouldCaptureRemoteKeys() {
				event := protocol.UIEvent{Kind: protocol.EventKindKey, Key: capturedKey}
				m.status = fmt.Sprintf("Sending %s event...", sanitizeChromeText(string(event.Kind)))
				return m, sendEventCmd(m.conn, m.sessionID, m.activeDoorID, m.renderRevision, event)
			}
		default:
			if m.focus == focusRemote && len(msg.Runes) > 0 {
				if m.shouldCaptureRemoteKeys() {
					event := protocol.UIEvent{Kind: protocol.EventKindKey, Key: capturedKey}
					m.status = fmt.Sprintf("Sending %s event...", sanitizeChromeText(string(event.Kind)))
					return m, sendEventCmd(m.conn, m.sessionID, m.activeDoorID, m.renderRevision, event)
				}
				m.appendRemoteInput(string(msg.Runes))
			} else if m.shouldCaptureRemoteKeys() {
				event := protocol.UIEvent{Kind: protocol.EventKindKey, Key: capturedKey}
				m.status = fmt.Sprintf("Sending %s event...", sanitizeChromeText(string(event.Kind)))
				return m, sendEventCmd(m.conn, m.sessionID, m.activeDoorID, m.renderRevision, event)
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampRemoteScroll()
	case protocol.RenderMessage:
		if !m.allowRender(time.Now()) {
			m.lastError = "Node exceeded render rate limit."
			m.status = "Disconnected for abuse."
			return m, tea.Quit
		}
		wasAtBottom := m.isRemoteScrolledToBottom()
		wasOpeningDoor := m.openingDoor != ""
		m.sessionID = msg.SessionID
		m.activeDoorID = msg.ActiveDoorID
		m.renderRevision = msg.RenderRevision
		m.currentView = msg.View
		if wasOpeningDoor {
			m.remoteScroll = 0
		}
		m.refreshRemoteState()
		m.clampRemoteScroll()
		if msg.View.Scroll == "bottom" && (wasOpeningDoor || wasAtBottom) {
			m.scrollRemoteToBottom()
		}
		if len(m.remoteState.focusables) == 0 && m.focus == focusRemote {
			m.focus = focusDoors
		}
		if wasOpeningDoor {
			m.status = fmt.Sprintf("Opened %s.", sanitizeChromeText(m.openingDoor))
			m.openingDoor = ""
		}
		m.lastError = ""
	case protocol.DoorListMessage:
		selectedDoorID := ""
		if m.selected >= 0 && m.selected < len(m.doors) {
			selectedDoorID = m.doors[m.selected].ID
		}
		m.doors = filterVisibleDoors(msg.Doors)
		if selectedDoorID != "" {
			m.selected = preferredDoorIndex(m.doors, selectedDoorID)
		}
		if m.selected >= len(m.doors) {
			m.selected = maxInt(len(m.doors)-1, 0)
		}
	case protocol.NotifyMessage:
		if !m.allowNotification(time.Now()) {
			m.lastError = "Node exceeded notification rate limit."
			m.status = "Disconnected for abuse."
			return m, tea.Quit
		}
		m.status = sanitizeChromeText(msg.Message)
	case protocol.ErrorMessage:
		if msg.Code != "" {
			m.lastError = sanitizeChromeText(fmt.Sprintf("%s: %s", msg.Code, msg.Message))
		} else {
			m.lastError = sanitizeChromeText(msg.Message)
		}
		m.status = "Node reported an error."
		m.openingDoor = ""
	case doorOpenedMsg:
		// render will usually arrive next; this mainly confirms the write succeeded.
	case eventSentMsg:
		// render or notify will typically arrive next.
	case connectionClosedMsg:
		m.status = msg.reason
		return m, tea.Quit
	case errMsg:
		m.lastError = msg.err.Error()
		m.status = "Connection error."
		return m, tea.Quit
	}
	return m, nil
}

func capturedKeyValue(msg tea.KeyMsg) string {
	if len(msg.Runes) == 1 {
		return string(msg.Runes[0])
	}
	return msg.String()
}

func (m tuiModel) View() string {
	appBackground := lipgloss.Color("#0D1117")
	panelBackground := lipgloss.Color("#1B1D21")

	appStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Foreground(lipgloss.Color("#E8F0F2")).
		Background(appBackground)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#9BE9A8"))

	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B949E"))

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3A4A5A")).
		BorderBackground(panelBackground).
		Background(panelBackground).
		Padding(1, 2)

	focusedPanel := panelStyle.Copy().
		BorderForeground(lipgloss.Color("#58A6FF"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E3B341"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF7B72"))

	if m.fullScreen {
		return m.fullScreenView(appStyle, headerStyle, metaStyle, panelStyle, statusStyle, errorStyle)
	}

	sidebarWidth := 28
	mainWidth := maxInt(m.width-sidebarWidth-10, 48)
	bodyHeight := maxInt(m.height-8, 10)
	barWidth := maxInt(m.width-4, 20)
	remoteView := m.renderRemoteView()
	visibleRemote, _ := scrollableText(remoteView, m.remoteScroll, m.remoteViewportHeight())

	headerLineStyle := lipgloss.NewStyle().
		Width(barWidth).
		Background(appBackground)
	titleLineStyle := headerLineStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#9BE9A8"))
	metaLineStyle := headerLineStyle.Copy().
		Foreground(lipgloss.Color("#8B949E"))
	header := lipgloss.JoinVertical(
		lipgloss.Left,
		titleLineStyle.Render("PhosphorNet"),
		metaLineStyle.Render(fmt.Sprintf("%-18s  %-48s  %s",
			"Node: "+sanitizeChromeText(m.auth.NodeName),
			"Node ID: "+sanitizeChromeText(m.auth.NodeID),
			"You: "+sanitizeChromeText(m.auth.Fingerprint),
		)),
	)

	doorRows := make([]string, 0, len(m.doors))
	for index, door := range m.doors {
		cursor := " "
		if index == m.selected {
			cursor = ">"
		}
		doorRows = append(doorRows, fmt.Sprintf("%s %s", cursor, sanitizeChromeText(door.Name)))
	}
	if len(doorRows) == 0 {
		doorRows = append(doorRows, "No visible doors.")
	}
	doorPanelStyle := panelStyle.Copy().Width(sidebarWidth)
	if m.focus == focusDoors {
		doorPanelStyle = focusedPanel.Copy().Width(sidebarWidth)
	}
	doorContentHeight := maxInt(bodyHeight-5, 3)
	visibleDoors, doorScrollLabel := scrollableText(strings.Join(doorRows, "\n"), m.doorScroll, doorContentHeight)
	doorTitle := "Doors"
	if doorScrollLabel != "" {
		doorTitle += " " + doorScrollLabel
	}
	doorsPanel := doorPanelStyle.
		Height(bodyHeight).
		Render(doorTitle + "\n\n" + visibleDoors)

	viewportPanelStyle := panelStyle.Copy().UnsetPadding()
	if m.focus == focusRemote {
		viewportPanelStyle = focusedPanel.Copy().UnsetPadding()
	}
	viewportPanel := viewportPanelStyle.
		Width(mainWidth).
		Height(bodyHeight).
		Render(visibleRemote)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left, viewportPanel)

	body := lipgloss.JoinHorizontal(lipgloss.Top, doorsPanel, rightColumn)

	statusLineStyle := headerLineStyle.Copy().
		Foreground(lipgloss.Color("#E3B341"))
	errorLineStyle := headerLineStyle.Copy().
		Foreground(lipgloss.Color("#FF7B72"))
	statusLines := []string{
		statusLineStyle.Render(sanitizeChromeText(m.status)),
		metaLineStyle.Render("Keys: f fullscreen, tab chrome, left/right select, up/down scroll, pgup/pgdn page, enter next/act, q quit."),
	}
	if m.lastError != "" {
		statusLines = append(statusLines, errorLineStyle.Render("Error: "+sanitizeChromeText(m.lastError)))
	}

	return appStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		body,
		"",
		strings.Join(statusLines, "\n"),
	))
}

func (m tuiModel) fullScreenView(appStyle, headerStyle, metaStyle, panelStyle, statusStyle, errorStyle lipgloss.Style) string {
	mainWidth := maxInt(m.width-8, 60)
	bodyHeight := maxInt(m.height-8, 12)
	remoteView := m.renderRemoteView()
	visibleRemote, _ := scrollableText(remoteView, m.remoteScroll, m.remoteViewportHeight())

	viewportPanel := panelStyle.Copy().
		UnsetPadding().
		Width(mainWidth).
		Height(bodyHeight).
		BorderForeground(lipgloss.Color("#58A6FF")).
		Render(visibleRemote)

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		headerStyle.Render("PhosphorNet"),
		"  ",
		metaStyle.Render(fmt.Sprintf("Node: %s", sanitizeChromeText(m.auth.NodeName))),
		"  ",
		metaStyle.Render(fmt.Sprintf("You: %s", sanitizeChromeText(m.auth.Fingerprint))),
		"  ",
		metaStyle.Render("Fullscreen door chrome"),
	)

	statusLines := []string{
		statusStyle.Render(sanitizeChromeText(m.status)),
		metaStyle.Render("Keys: f/esc station view, left/right select, up/down scroll, pgup/pgdn page, enter next/act, q quit."),
	}
	if m.lastError != "" {
		statusLines = append(statusLines, errorStyle.Render("Error: "+sanitizeChromeText(m.lastError)))
	}

	return appStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		viewportPanel,
		"",
		strings.Join(statusLines, "\n"),
	))
}

func (m *tuiModel) refreshRemoteState() {
	_, state := m.renderRemote()
	m.remoteState = state
	if m.focusedRemote >= len(m.remoteState.focusables) {
		m.focusedRemote = maxInt(len(m.remoteState.focusables)-1, 0)
	}
}

func (m tuiModel) renderRemoteView() string {
	rendered, _ := m.renderRemote()
	return rendered
}

func (m tuiModel) renderRemote() (string, renderState) {
	remoteFocus := -1
	if m.focus == focusRemote {
		remoteFocus = m.focusedRemote
	}
	return RenderComponents(m.currentView, renderOptions{
		Width:          m.remoteRenderWidth(),
		ViewportHeight: m.remoteViewportHeight(),
		FocusedIndex:   remoteFocus,
		ItemSelection:  m.itemSelection,
		InputValues:    m.inputValues,
	})
}

func (m *tuiModel) allowRender(now time.Time) bool {
	if m.renderWindowStart.IsZero() || now.Sub(m.renderWindowStart) >= time.Second {
		m.renderWindowStart = now
		m.renderWindowCount = 0
	}
	m.renderWindowCount++
	return m.renderWindowCount <= protocol.MaxRenderMessagesPerSecond
}

func (m *tuiModel) allowNotification(now time.Time) bool {
	if m.notifyWindowStart.IsZero() || now.Sub(m.notifyWindowStart) >= time.Minute {
		m.notifyWindowStart = now
		m.notifyWindowCount = 0
	}
	m.notifyWindowCount++
	return m.notifyWindowCount <= protocol.MaxNotificationsPerMinute
}

func (m tuiModel) remoteRenderWidth() int {
	if m.fullScreen {
		return maxInt(m.width-8, 48)
	}
	sidebarWidth := 28
	mainWidth := maxInt(m.width-sidebarWidth-10, 48)
	return maxInt(mainWidth, 36)
}

func (m tuiModel) remoteViewportHeight() int {
	if m.fullScreen {
		return maxInt(maxInt(m.height-8, 12), 6)
	}
	return maxInt(maxInt(m.height-8, 10), 4)
}

func (m tuiModel) remotePageSize() int {
	return maxInt(m.remoteViewportHeight()-2, 1)
}

func (m tuiModel) doorViewportHeight() int {
	return maxInt(maxInt(m.height-8, 10)-5, 3)
}

func (m *tuiModel) moveDoorSelection(delta int) bool {
	if len(m.doors) == 0 || delta == 0 {
		return false
	}
	next := m.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.doors) {
		next = len(m.doors) - 1
	}
	if next == m.selected {
		return false
	}
	m.selected = next
	m.ensureSelectedDoorVisible()
	return true
}

func (m *tuiModel) scrollDoors(delta int) bool {
	if delta == 0 {
		return false
	}
	previous := m.doorScroll
	m.doorScroll += delta
	m.clampDoorScroll()
	return m.doorScroll != previous
}

func (m *tuiModel) clampDoorScroll() {
	maxScroll := m.maxDoorScroll()
	if m.doorScroll < 0 {
		m.doorScroll = 0
	}
	if m.doorScroll > maxScroll {
		m.doorScroll = maxScroll
	}
}

func (m tuiModel) maxDoorScroll() int {
	return maxInt(len(m.doors)-m.doorViewportHeight(), 0)
}

func (m *tuiModel) ensureSelectedDoorVisible() {
	if len(m.doors) == 0 {
		m.doorScroll = 0
		return
	}
	if m.selected < m.doorScroll {
		m.doorScroll = m.selected
	}
	bottom := m.doorScroll + m.doorViewportHeight() - 1
	if m.selected > bottom {
		m.doorScroll = m.selected - m.doorViewportHeight() + 1
	}
	m.clampDoorScroll()
}

func (m *tuiModel) scrollRemote(delta int) bool {
	previous := m.remoteScroll
	m.remoteScroll += delta
	m.clampRemoteScroll()
	return m.remoteScroll != previous
}

func (m *tuiModel) clampRemoteScroll() {
	maxScroll := m.maxRemoteScroll()
	if m.remoteScroll < 0 {
		m.remoteScroll = 0
	}
	if m.remoteScroll > maxScroll {
		m.remoteScroll = maxScroll
	}
}

func (m tuiModel) maxRemoteScroll() int {
	return maxInt(renderedLineCount(m.renderRemoteView())-m.remoteViewportHeight(), 0)
}

func (m tuiModel) isRemoteScrolledToBottom() bool {
	return m.remoteScroll >= m.maxRemoteScroll()
}

func (m *tuiModel) scrollRemoteToBottom() {
	m.remoteScroll = m.maxRemoteScroll()
}

func (m *tuiModel) focusedComponent() (focusableComponent, bool) {
	if m.focusedRemote < 0 || m.focusedRemote >= len(m.remoteState.focusables) {
		return focusableComponent{}, false
	}
	return m.remoteState.focusables[m.focusedRemote], true
}

func (m *tuiModel) clearRemoteFocus() {
	m.focusedRemote = -1
}

func (m *tuiModel) focusedComponentIsChoice() bool {
	component, ok := m.focusedComponent()
	return ok && (component.Kind == focusableMenu || component.Kind == focusableList)
}

func (m *tuiModel) focusedComponentIsTextInput() bool {
	component, ok := m.focusedComponent()
	return ok && (component.Kind == focusableInput || component.Kind == focusableTextarea)
}

func (m *tuiModel) focusedComponentIsTextarea() bool {
	component, ok := m.focusedComponent()
	return ok && component.Kind == focusableTextarea
}

func (m *tuiModel) focusedComponentSubmitsOnEnter() bool {
	component, ok := m.focusedComponent()
	if !ok {
		return false
	}
	return component.Kind == focusableInput && component.Dock == "bottom"
}

func (m *tuiModel) shouldCaptureRemoteKeys() bool {
	if m.focus != focusRemote || !m.currentView.CaptureKeys {
		return false
	}
	return !m.focusedComponentIsTextInput()
}

func (m *tuiModel) moveRemoteSelection(delta int) {
	component, ok := m.focusedComponent()
	if !ok {
		return
	}
	switch component.Kind {
	case focusableMenu, focusableList:
		if component.ItemCount == 0 {
			return
		}
		next := m.itemSelection[component.ID] + delta
		if next < 0 {
			next = 0
		}
		if next >= component.ItemCount {
			next = component.ItemCount - 1
		}
		m.itemSelection[component.ID] = next
	}
}

func (m *tuiModel) moveRemoteFocus(delta int) bool {
	visible := m.visibleRemoteFocusables()
	if len(visible) == 0 {
		return false
	}

	currentVisible := -1
	for index, focusIndex := range visible {
		if focusIndex == m.focusedRemote {
			currentVisible = index
			break
		}
	}
	if currentVisible == -1 {
		if delta < 0 {
			m.focusedRemote = visible[len(visible)-1]
		} else {
			m.focusedRemote = visible[0]
		}
		return true
	}

	next := currentVisible + delta
	if next < 0 || next >= len(visible) {
		return false
	}
	m.focusedRemote = visible[next]
	return true
}

func (m *tuiModel) moveRemoteFocusCycle(delta int) bool {
	visible := m.visibleRemoteFocusables()
	if len(visible) == 0 {
		return false
	}
	if delta == 0 {
		return true
	}

	currentVisible := -1
	for index, focusIndex := range visible {
		if focusIndex == m.focusedRemote {
			currentVisible = index
			break
		}
	}
	if currentVisible == -1 {
		if delta < 0 {
			m.focusedRemote = visible[len(visible)-1]
		} else {
			m.focusedRemote = visible[0]
		}
		return true
	}

	next := (currentVisible + delta) % len(visible)
	if next < 0 {
		next += len(visible)
	}
	m.focusedRemote = visible[next]
	return true
}

func (m tuiModel) visibleRemoteFocusables() []int {
	lines := m.remoteFocusableLines()
	visible := []int{}
	start := m.remoteScroll
	end := start + m.remoteViewportHeight()
	for index, line := range lines {
		if line >= start && line < end {
			visible = append(visible, index)
		}
	}
	return visible
}

func (m tuiModel) remoteFocusableLines() []int {
	if len(m.remoteState.focusables) == 0 {
		return nil
	}
	base := splitRenderableLines(m.renderRemoteWithFocus(-1))
	lines := make([]int, len(m.remoteState.focusables))
	for index := range lines {
		lines[index] = -1
		focused := splitRenderableLines(m.renderRemoteWithFocus(index))
		limit := minInt(len(base), len(focused))
		for line := 0; line < limit; line++ {
			if base[line] != focused[line] {
				lines[index] = line
				break
			}
		}
		if lines[index] == -1 && len(focused) != len(base) {
			lines[index] = limit
		}
	}
	return lines
}

func (m tuiModel) renderRemoteWithFocus(focusedIndex int) string {
	rendered, _ := RenderComponents(m.currentView, renderOptions{
		Width:          m.remoteRenderWidth(),
		ViewportHeight: m.remoteViewportHeight(),
		FocusedIndex:   focusedIndex,
		ItemSelection:  m.itemSelection,
		InputValues:    m.inputValues,
	})
	return rendered
}

func (m *tuiModel) appendRemoteInput(value string) {
	component, ok := m.focusedComponent()
	if !ok {
		return
	}
	if component.Kind != focusableInput && component.Kind != focusableTextarea {
		return
	}
	m.inputValues[component.ID] += value
}

func (m *tuiModel) backspaceRemoteInput() {
	component, ok := m.focusedComponent()
	if !ok {
		return
	}
	if component.Kind != focusableInput && component.Kind != focusableTextarea {
		return
	}
	value := []rune(m.inputValues[component.ID])
	if len(value) == 0 {
		return
	}
	m.inputValues[component.ID] = string(value[:len(value)-1])
}

func (m *tuiModel) remoteEventForFocus() (protocol.UIEvent, bool) {
	component, ok := m.focusedComponent()
	if !ok {
		return protocol.UIEvent{}, false
	}
	switch component.Kind {
	case focusableMenu:
		if component.ItemCount == 0 {
			return protocol.UIEvent{}, false
		}
		selected := m.itemSelection[component.ID]
		if selected >= len(component.Items) {
			selected = len(component.Items) - 1
		}
		item := component.Items[selected]
		return protocol.UIEvent{
			Kind:   protocol.EventKindAction,
			Target: component.ID,
			Action: item.Action,
		}, true
	case focusableList:
		if component.ItemCount == 0 {
			return protocol.UIEvent{}, false
		}
		selected := m.itemSelection[component.ID]
		if selected >= len(component.Items) {
			selected = len(component.Items) - 1
		}
		item := component.Items[selected]
		return protocol.UIEvent{
			Kind:   protocol.EventKindSelect,
			Target: component.ID,
			Action: item.Action,
			Values: map[string]string{"label": item.Label},
		}, true
	case focusableButton:
		values := copyStringMap(m.inputValues)
		if component.Label != "" {
			if values == nil {
				values = map[string]string{}
			}
			values["label"] = component.Label
		}
		return protocol.UIEvent{
			Kind:   protocol.EventKindAction,
			Target: component.ID,
			Action: component.Action,
			Values: values,
		}, true
	case focusableCheckbox:
		values := copyStringMap(m.inputValues)
		if values == nil {
			values = map[string]string{}
		}
		values["checked"] = fmt.Sprintf("%t", !component.Checked)
		return protocol.UIEvent{
			Kind:   protocol.EventKindAction,
			Target: component.ID,
			Action: component.Action,
			Values: values,
		}, true
	case focusableInput, focusableTextarea:
		return protocol.UIEvent{
			Kind:   protocol.EventKindSubmit,
			Target: component.ID,
			Values: map[string]string{component.ID: m.inputValues[component.ID]},
		}, true
	default:
		return protocol.UIEvent{}, false
	}
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func openDoorCmd(conn *websocket.Conn, doorID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wsjson.Write(ctx, conn, protocol.OpenDoorMessage{
			Type:   protocol.TypeOpenDoor,
			DoorID: doorID,
		}); err != nil {
			return errMsg{err: err}
		}
		return doorOpenedMsg{doorID: doorID}
	}
}

func sendEventCmd(conn *websocket.Conn, sessionID, activeDoorID string, renderRevision int64, event protocol.UIEvent) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wsjson.Write(ctx, conn, protocol.EventMessage{
			Type:           protocol.TypeEvent,
			SessionID:      sessionID,
			ActiveDoorID:   activeDoorID,
			RenderRevision: renderRevision,
			EventID:        newEventID(),
			Event:          event,
		}); err != nil {
			return errMsg{err: err}
		}
		return eventSentMsg{}
	}
}

func newEventID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("event-%d", time.Now().UnixNano())
}

func readRawMessage(ctx context.Context, conn *websocket.Conn) (tea.Msg, error) {
	var raw json.RawMessage
	if err := wsjson.Read(ctx, conn, &raw); err != nil {
		return nil, err
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode message envelope: %w", err)
	}

	switch envelope.Type {
	case protocol.TypeRender:
		var message protocol.RenderMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode render message: %w", err)
		}
		return message, nil
	case protocol.TypeDoorList:
		var message protocol.DoorListMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode door list message: %w", err)
		}
		return message, nil
	case protocol.TypeNotify:
		var message protocol.NotifyMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode notify message: %w", err)
		}
		return message, nil
	case protocol.TypeError:
		var message protocol.ErrorMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("decode error message: %w", err)
		}
		return message, nil
	default:
		return protocol.NotifyMessage{
			Type:    protocol.TypeNotify,
			Level:   "info",
			Message: fmt.Sprintf("Unhandled message type %q", envelope.Type),
		}, nil
	}
}

func preferredDoorIndex(doors []protocol.DoorSummary, preferredID string) int {
	for index, door := range doors {
		if door.ID == preferredID {
			return index
		}
	}
	return 0
}

func filterVisibleDoors(doors []protocol.DoorSummary) []protocol.DoorSummary {
	visible := make([]protocol.DoorSummary, 0, len(doors))
	for _, door := range doors {
		switch strings.ToLower(strings.TrimSpace(door.Visibility)) {
		case "", "public":
			visible = append(visible, door)
		}
	}
	return visible
}

func scrollableText(value string, offset, height int) (string, string) {
	lines := splitRenderableLines(value)
	if height <= 0 {
		return "", ""
	}
	if len(lines) <= height {
		return strings.Join(lines, "\n"), ""
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines)-height {
		offset = len(lines) - height
	}
	end := offset + height
	label := fmt.Sprintf("[%d-%d/%d]", offset+1, end, len(lines))
	return strings.Join(lines[offset:end], "\n"), label
}

func clipLines(value string, offset, height int) string {
	lines := splitRenderableLines(value)
	if height <= 0 || len(lines) == 0 {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return ""
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[offset:end], "\n")
}

func renderedLineCount(value string) int {
	return len(splitRenderableLines(value))
}

func splitRenderableLines(value string) []string {
	value = strings.TrimRight(value, "\n")
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
