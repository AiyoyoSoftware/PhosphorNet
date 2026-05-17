package client

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"phosphornet/internal/protocol"
)

type focusableKind string

const (
	focusableMenu     focusableKind = "menu"
	focusableButton   focusableKind = "button"
	focusableCheckbox focusableKind = "checkbox"
	focusableInput    focusableKind = "input"
	focusableTextarea focusableKind = "textarea"
	focusableList     focusableKind = "list"
)

type focusableComponent struct {
	Kind      focusableKind
	ID        string
	Action    string
	Label     string
	Dock      string
	Checked   bool
	Items     []protocol.Item
	ItemCount int
}

type renderState struct {
	focusables []focusableComponent
}

type renderOptions struct {
	Width          int
	ViewportHeight int
	FocusedIndex   int
	ItemSelection  map[string]int
	InputValues    map[string]string
	Constrained    bool
}

type componentStyles struct {
	header      lipgloss.Style
	text        lipgloss.Style
	status      lipgloss.Style
	panel       lipgloss.Style
	panelTitle  lipgloss.Style
	focused     lipgloss.Style
	menuItem    lipgloss.Style
	selected    lipgloss.Style
	input       lipgloss.Style
	inputFocus  lipgloss.Style
	button      lipgloss.Style
	buttonFocus lipgloss.Style
	grid        lipgloss.Style
	log         lipgloss.Style
	muted       lipgloss.Style
}

type resolvedBackground struct {
	kind      string
	direction string
	stops     []resolvedGradientStop
}

type resolvedGradientStop struct {
	at    float64
	color lipgloss.Color
}

func RenderComponents(node protocol.UINode, options renderOptions) (string, renderState) {
	state := renderState{}
	styles := newComponentStyles()
	rendered := renderComponent(sanitizeRemoteNode(node, 0), options, styles, &state)
	return strings.TrimSpace(rendered), state
}

func RenderText(node protocol.UINode) string {
	rendered, _ := RenderComponents(node, renderOptions{Width: 80, FocusedIndex: -1})
	return rendered
}

func renderComponent(node protocol.UINode, options renderOptions, styles componentStyles, state *renderState) string {
	switch node.Component {
	case "screen":
		if background, ok := resolveContainerBackground(node.Style); ok {
			localStyles := styles.withTransparentBackgrounds()
			rendered := ""
			if shouldStickLastInputToBottom(node, options) {
				rendered = renderBottomPinnedScreen(node, options, localStyles, state)
			} else {
				rendered = renderChildren(node.Children, options, localStyles, state)
			}
			return applyBackground(rendered, background, options.Width, options.ViewportHeight)
		}
		if shouldStickLastInputToBottom(node, options) {
			return renderBottomPinnedScreen(node, options, styles, state)
		}
		return renderChildren(node.Children, options, styles, state)
	case "header":
		return styles.header.Width(maxInt(options.Width-2, 20)).Render(node.Text)
	case "status":
		return styles.status.Width(maxInt(options.Width-2, 20)).Render(node.Text)
	case "text":
		return styles.text.Width(maxInt(options.Width-2, 20)).Render(node.Text)
	case "markdown":
		return renderMarkdown(node.Text, options.Width)
	case "panel":
		if background, ok := resolveContainerBackground(node.Style); ok {
			localStyles := styles.withTransparentBackgrounds()
			panelWidth := renderAvailableWidth(options, 4, 20)
			panelStyle := localStyles.panel.UnsetBackground().UnsetBorderBackground().UnsetMarginTop().UnsetPadding()
			panelStyleWidth := maxInt(panelWidth-panelStyle.GetHorizontalBorderSize(), 1)
			contentWidth := panelStyleWidth
			innerWidth := maxInt(contentWidth-4, 1)
			childOptions := options
			childOptions.Width = innerWidth
			childOptions.Constrained = true
			body := renderChildren(node.Children, childOptions, localStyles, state)
			title := localStyles.panelTitle.Render(node.Title)
			content := panelInteriorContent(strings.TrimSpace(lipgloss.JoinVertical(lipgloss.Left, title, body)), contentWidth)
			content = applyBackground(content, background, contentWidth, 0)
			panel := panelStyle.Width(panelStyleWidth).Render(content)
			return panel
		}
		panelWidth := renderAvailableWidth(options, 4, 20)
		panelStyleWidth := maxInt(panelWidth-styles.panel.GetHorizontalBorderSize(), 1)
		innerWidth := maxInt(panelStyleWidth-styles.panel.GetHorizontalPadding(), 1)
		childOptions := options
		childOptions.Width = innerWidth
		childOptions.Constrained = true
		body := renderChildren(node.Children, childOptions, styles, state)
		title := styles.panelTitle.Render(node.Title)
		content := strings.TrimSpace(lipgloss.JoinVertical(lipgloss.Left, title, body))
		return styles.panel.Width(panelStyleWidth).Render(content)
	case "menu":
		focusIndex := addFocusable(state, focusableComponent{
			Kind:      focusableMenu,
			ID:        node.ID,
			Items:     node.Items,
			ItemCount: len(node.Items),
		})
		return renderItems(node, options, styles, focusIndex)
	case "list":
		focusIndex := addFocusable(state, focusableComponent{
			Kind:      focusableList,
			ID:        node.ID,
			Items:     node.Items,
			ItemCount: len(node.Items),
		})
		return renderItems(node, options, styles, focusIndex)
	case "dynamic_list":
		return renderDynamicList(node, options, styles, state)
	case "input", "textarea":
		kind := focusableInput
		if node.Component == "textarea" {
			kind = focusableTextarea
		}
		focusIndex := addFocusable(state, focusableComponent{Kind: kind, ID: node.ID, Dock: node.Dock})
		return renderInput(node, options, styles, focusIndex)
	case "button":
		focusIndex := addFocusable(state, focusableComponent{
			Kind:   focusableButton,
			ID:     node.ID,
			Action: node.Action,
		})
		return renderButton(node, options, styles, focusIndex)
	case "checkbox":
		focusIndex := addFocusable(state, focusableComponent{
			Kind:    focusableCheckbox,
			ID:      node.ID,
			Action:  node.Action,
			Checked: node.Checked,
		})
		return renderCheckbox(node, options, styles, focusIndex)
	case "grid":
		return renderGrid(node, options, styles)
	case "log":
		if background, ok := resolveContainerBackground(node.Style); ok {
			localStyles := styles.withTransparentBackgrounds()
			body := renderChildren(node.Children, options, localStyles, state)
			logStyle := localStyles.log.UnsetBackground().UnsetBorderBackground()
			return applyBackground(logStyle.Width(maxInt(options.Width-2, 20)).Render(body), background, maxInt(options.Width-2, 20), 0)
		}
		body := renderChildren(node.Children, options, styles, state)
		return styles.log.Width(maxInt(options.Width-2, 20)).Render(body)
	default:
		return styles.muted.Render("[unsupported component]")
	}
}

func shouldStickLastInputToBottom(node protocol.UINode, options renderOptions) bool {
	if node.Scroll != "bottom" || options.ViewportHeight <= 0 || len(node.Children) == 0 {
		return false
	}
	last := node.Children[len(node.Children)-1]
	return last.Dock == "bottom" && (last.Component == "input" || last.Component == "textarea")
}

func renderBottomPinnedScreen(node protocol.UINode, options renderOptions, styles componentStyles, state *renderState) string {
	bodyChildren := node.Children[:len(node.Children)-1]
	footer := node.Children[len(node.Children)-1]
	body := strings.TrimSpace(renderChildren(bodyChildren, options, styles, state))
	footerRendered := strings.TrimSpace(renderComponent(footer, options, styles, state))
	if footerRendered == "" {
		return body
	}
	bodyLines := 0
	if body != "" {
		bodyLines = renderedLineCount(body)
	}
	footerLines := renderedLineCount(footerRendered)
	if body == "" {
		leadingNewlines := maxInt(options.ViewportHeight-footerLines, 0)
		return strings.Repeat("\n", leadingNewlines) + footerRendered
	}
	newlinesBeforeFooter := maxInt(options.ViewportHeight-bodyLines-footerLines+1, 1)
	return body + strings.Repeat("\n", newlinesBeforeFooter) + footerRendered
}

func renderChildren(children []protocol.UINode, options renderOptions, styles componentStyles, state *renderState) string {
	limit := minInt(len(children), protocol.MaxUIChildren)
	parts := make([]string, 0, limit)
	for index := 0; index < limit; index++ {
		child := children[index]
		rendered := renderComponent(child, options, styles, state)
		if strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	if len(children) > limit {
		parts = append(parts, styles.muted.Render("[content truncated]"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderItems(node protocol.UINode, options renderOptions, styles componentStyles, focusIndex int) string {
	if len(node.Items) == 0 {
		return styles.muted.Render("No choices.")
	}
	limit := minInt(len(node.Items), protocol.MaxUIItems)
	selected := options.ItemSelection[node.ID]
	if selected >= limit {
		selected = limit - 1
	}
	rows := make([]string, 0, limit)
	for index := 0; index < limit; index++ {
		item := node.Items[index]
		prefix := "  "
		style := styles.menuItem
		if options.FocusedIndex == focusIndex && index == selected {
			prefix = "> "
			style = styles.selected
		}
		rows = append(rows, style.Render(prefix+item.Label))
	}
	if len(node.Items) > limit {
		rows = append(rows, styles.muted.Render("  [more choices truncated]"))
	}
	return strings.Join(rows, "\n")
}

func renderDynamicList(node protocol.UINode, options renderOptions, styles componentStyles, state *renderState) string {
	if len(node.Items) == 0 {
		return styles.muted.Render("No items.")
	}
	limit := minInt(len(node.Items), protocol.MaxUIItems)
	rows := make([]string, 0, limit)
	for index := 0; index < limit; index++ {
		item := node.Items[index]
		focusIndex := addFocusable(state, focusableComponent{
			Kind:   focusableButton,
			ID:     node.ID,
			Action: item.Action,
			Label:  item.Label,
		})
		prefix := "  "
		style := styles.menuItem
		if options.FocusedIndex == focusIndex {
			prefix = "> "
			style = styles.selected
		}
		rows = append(rows, style.Render(prefix+item.Label))
	}
	if len(node.Items) > limit {
		rows = append(rows, styles.muted.Render("  [more items truncated]"))
	}
	return strings.Join(rows, "\n")
}

func renderInput(node protocol.UINode, options renderOptions, styles componentStyles, focusIndex int) string {
	renderWidth := renderAvailableWidth(options, 4, 20)
	value, ok := options.InputValues[node.ID]
	if !ok {
		value = node.Value
	}
	if value == "" {
		value = node.Placeholder
	}
	style := styles.input
	prefix := "  "
	if options.FocusedIndex == focusIndex {
		style = styles.inputFocus
		prefix = "> "
		contentWidth := maxInt(renderWidth-style.GetHorizontalFrameSize(), 1)
		maxContentRunes := maxInt(contentWidth-len([]rune(prefix)), 0)
		if len([]rune(value)) < maxContentRunes {
			value += " "
		}
	}
	return style.Width(renderWidth).Render(prefix + value)
}

func renderAvailableWidth(options renderOptions, gutter int, minimum int) int {
	if options.Constrained {
		return maxInt(options.Width, 1)
	}
	return maxInt(options.Width-gutter, minimum)
}

func panelInteriorContent(content string, width int) string {
	if width <= 0 {
		return content
	}
	innerWidth := maxInt(width-4, 1)
	lines := []string{strings.Repeat(" ", width)}
	for _, line := range strings.Split(content, "\n") {
		lines = append(lines, "  "+padRenderedLine(line, innerWidth)+"  ")
	}
	lines = append(lines, strings.Repeat(" ", width))
	return strings.Join(lines, "\n")
}

func renderButton(node protocol.UINode, options renderOptions, styles componentStyles, focusIndex int) string {
	style := styles.button
	label := "[ " + node.Text + " ]"
	if options.FocusedIndex == focusIndex {
		style = styles.buttonFocus
		label = "> " + label
	}
	return style.Render(label)
}

func renderCheckbox(node protocol.UINode, options renderOptions, styles componentStyles, focusIndex int) string {
	style := styles.button
	mark := " "
	if node.Checked {
		mark = "x"
	}
	label := fmt.Sprintf("[%s] %s", mark, node.Text)
	if options.FocusedIndex == focusIndex {
		style = styles.buttonFocus
		label = "> " + label
	}
	return style.Render(label)
}

func renderGrid(node protocol.UINode, options renderOptions, styles componentStyles) string {
	if len(node.Rows) == 0 {
		return styles.muted.Render("Empty grid.")
	}
	rowLimit := minInt(len(node.Rows), protocol.MaxGridRows)
	rows := make([]string, 0, rowLimit)
	for rowIndex := 0; rowIndex < rowLimit; rowIndex++ {
		row := node.Rows[rowIndex]
		cellLimit := minInt(len(row), protocol.MaxGridCols)
		cells := make([]string, 0, cellLimit)
		for cellIndex := 0; cellIndex < cellLimit; cellIndex++ {
			cell := row[cellIndex]
			cells = append(cells, fmt.Sprintf("%-3s", cell))
		}
		rows = append(rows, strings.TrimRight(strings.Join(cells, ""), " "))
	}
	if len(node.Rows) > rowLimit {
		rows = append(rows, "[grid truncated]")
	}
	return styles.grid.Width(maxInt(options.Width-4, 20)).Render(strings.Join(rows, "\n"))
}

func renderMarkdown(text string, width int) string {
	text = sanitizeMarkdownText(text)
	if text == "" {
		return ""
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(maxInt(width-4, 20)),
	)
	if err != nil {
		return text
	}

	rendered, err := renderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(rendered)
}

func addFocusable(state *renderState, component focusableComponent) int {
	if component.ID == "" {
		component.ID = fmt.Sprintf("_remote_%d", len(state.focusables))
	}
	state.focusables = append(state.focusables, component)
	return len(state.focusables) - 1
}

func newComponentStyles() componentStyles {
	panelBackground := lipgloss.Color("#1B1D21")
	return componentStyles{
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD166")).
			Background(panelBackground).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#6B5845")).
			BorderBackground(panelBackground).
			MarginBottom(1),
		text: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E8F0F2")).
			Background(panelBackground),
		status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9BE9A8")).
			Background(panelBackground).
			MarginTop(1),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2F5D62")).
			BorderBackground(panelBackground).
			Background(panelBackground).
			Padding(1, 2).
			MarginTop(1),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7BDFF2")).
			Background(panelBackground).
			MarginBottom(1),
		menuItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E8F0F2")).
			Background(panelBackground),
		selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0D1117")).
			Background(lipgloss.Color("#FFD166")),
		input: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			Background(lipgloss.Color("#161B22")).
			Padding(0, 1),
		inputFocus: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E8F0F2")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1),
		button: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E8F0F2")).
			Background(panelBackground),
		buttonFocus: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD166")).
			Background(panelBackground),
		grid: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C3F73A")).
			Background(lipgloss.Color("#111A12")).
			Padding(1, 2),
		log: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#3A4A5A")).
			BorderBackground(panelBackground).
			Background(panelBackground).
			PaddingTop(1),
		muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			Background(panelBackground),
	}
}

func (styles componentStyles) withTransparentBackgrounds() componentStyles {
	styles.header = styles.header.UnsetBackground().UnsetBorderBackground()
	styles.text = styles.text.UnsetBackground()
	styles.status = styles.status.UnsetBackground()
	styles.panel = styles.panel.UnsetBackground().UnsetBorderBackground()
	styles.panelTitle = styles.panelTitle.UnsetBackground()
	styles.menuItem = styles.menuItem.UnsetBackground()
	styles.button = styles.button.UnsetBackground()
	styles.buttonFocus = styles.buttonFocus.UnsetBackground()
	styles.log = styles.log.UnsetBackground().UnsetBorderBackground()
	styles.muted = styles.muted.UnsetBackground()
	return styles
}

func resolveContainerBackground(style *protocol.UIStyle) (resolvedBackground, bool) {
	if style == nil || style.Background == nil {
		return resolvedBackground{}, false
	}
	background := style.Background
	switch background.Kind {
	case "solid":
		color := firstValidHexColor(background.Color, background.From)
		if color == "" {
			return resolvedBackground{}, false
		}
		return resolvedBackground{
			kind: "solid",
			stops: []resolvedGradientStop{{
				at:    0,
				color: lipgloss.Color(color),
			}},
		}, true
	case "gradient":
		stops := resolveGradientStops(background)
		if len(stops) == 0 {
			return resolvedBackground{}, false
		}
		return resolvedBackground{
			kind:      "gradient",
			direction: normalizeGradientDirection(background.Direction),
			stops:     stops,
		}, true
	default:
		return resolvedBackground{}, false
	}
}

func resolveGradientStops(background *protocol.UIBackground) []resolvedGradientStop {
	if len(background.Stops) > 0 {
		stops := make([]resolvedGradientStop, 0, minInt(len(background.Stops), protocol.MaxUIGradientStops))
		for index := 0; index < len(background.Stops) && index < protocol.MaxUIGradientStops; index++ {
			stop := background.Stops[index]
			if !isValidHexColor(stop.Color) {
				continue
			}
			stops = append(stops, resolvedGradientStop{
				at:    clampFloat(stop.At, 0, 1),
				color: lipgloss.Color(stop.Color),
			})
		}
		sort.SliceStable(stops, func(i, j int) bool {
			return stops[i].at < stops[j].at
		})
		return stops
	}

	from := firstValidHexColor(background.From, background.Color)
	to := firstValidHexColor(background.To, background.From, background.Color)
	if from == "" || to == "" {
		return nil
	}
	return []resolvedGradientStop{
		{at: 0, color: lipgloss.Color(from)},
		{at: 1, color: lipgloss.Color(to)},
	}
}

func normalizeGradientDirection(value string) string {
	switch value {
	case "horizontal", "diagonal":
		return value
	default:
		return "vertical"
	}
}

func firstValidHexColor(values ...string) string {
	for _, value := range values {
		if isValidHexColor(value) {
			return value
		}
	}
	return ""
}

func isValidHexColor(value string) bool {
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

func applyBackground(rendered string, background resolvedBackground, width, height int) string {
	if len(background.stops) == 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if rendered == "" && width > 0 && height > 0 {
		lines = nil
	}
	if width > 0 {
		for index := range lines {
			lines[index] = padRenderedLine(lines[index], width)
		}
	}
	if height > len(lines) && width > 0 {
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}
	lineCount := len(lines)
	result := make([]string, 0, lineCount)
	for row, line := range lines {
		color := backgroundColorAt(background, row, lineCount, lipgloss.Width(line))
		result = append(result, paintLineBackground(line, color))
	}
	return strings.Join(result, "\n")
}

func padRenderedLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func paintLineBackground(line string, color lipgloss.Color) string {
	red, green, blue, ok := parseHexColor(string(color))
	if !ok {
		return lipgloss.NewStyle().Background(color).Render(line)
	}
	backgroundSeq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", red, green, blue)
	var out strings.Builder
	out.Grow(len(line) + len(backgroundSeq)*2 + len("\x1b[0m"))
	out.WriteString(backgroundSeq)
	for index := 0; index < len(line); {
		if line[index] != '\x1b' || index+1 >= len(line) || line[index+1] != '[' {
			out.WriteByte(line[index])
			index++
			continue
		}
		end := index + 2
		for end < len(line) {
			if line[end] >= 0x40 && line[end] <= 0x7e {
				end++
				break
			}
			end++
		}
		sequence := line[index:end]
		out.WriteString(sequence)
		if shouldReapplyBackground(sequence) {
			out.WriteString(backgroundSeq)
		}
		index = end
	}
	out.WriteString("\x1b[0m")
	return out.String()
}

func shouldReapplyBackground(sequence string) bool {
	if !strings.HasSuffix(sequence, "m") || !strings.HasPrefix(sequence, "\x1b[") {
		return false
	}
	params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
	if params == "" {
		return true
	}
	parts := strings.FieldsFunc(params, func(r rune) bool {
		return r == ';' || r == ':'
	})
	for _, part := range parts {
		if part == "48" || part == "49" || (len(part) == 2 && part[0] == '4') || (len(part) == 3 && part[0] == '1' && part[1] == '0') {
			return part == "49"
		}
	}
	return true
}

func backgroundColorAt(background resolvedBackground, row, lineCount, lineWidth int) lipgloss.Color {
	if background.kind == "solid" || len(background.stops) == 1 {
		return background.stops[0].color
	}
	rowPosition := gradientRatio(row, maxInt(lineCount-1, 1))
	switch background.direction {
	case "horizontal":
		return gradientColorAt(background.stops, 0.5)
	case "diagonal":
		columnPosition := 0.5
		if lineWidth <= 1 {
			columnPosition = 0
		}
		return gradientColorAt(background.stops, (rowPosition+columnPosition)/2)
	default:
		return gradientColorAt(background.stops, rowPosition)
	}
}

func gradientRatio(index, maxIndex int) float64 {
	if maxIndex <= 0 {
		return 0
	}
	return float64(index) / float64(maxIndex)
}

func gradientColorAt(stops []resolvedGradientStop, position float64) lipgloss.Color {
	position = clampFloat(position, 0, 1)
	if position <= stops[0].at {
		return stops[0].color
	}
	last := stops[len(stops)-1]
	if position >= last.at {
		return last.color
	}
	for index := 1; index < len(stops); index++ {
		right := stops[index]
		if position > right.at {
			continue
		}
		left := stops[index-1]
		span := right.at - left.at
		if span <= 0 {
			return right.color
		}
		return mixHexColors(left.color, right.color, (position-left.at)/span)
	}
	return last.color
}

func mixHexColors(left, right lipgloss.Color, amount float64) lipgloss.Color {
	lr, lg, lb, ok := parseHexColor(string(left))
	if !ok {
		return right
	}
	rr, rg, rb, ok := parseHexColor(string(right))
	if !ok {
		return left
	}
	amount = clampFloat(amount, 0, 1)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x",
		mixChannel(lr, rr, amount),
		mixChannel(lg, rg, amount),
		mixChannel(lb, rb, amount),
	))
}

func parseHexColor(value string) (int, int, int, bool) {
	if !isValidHexColor(value) {
		return 0, 0, 0, false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0, false
	}
	return r, g, b, true
}

func mixChannel(left, right int, amount float64) int {
	return int(math.Round(float64(left) + (float64(right-left) * amount)))
}

func clampFloat(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func sanitizeRemoteNode(node protocol.UINode, depth int) protocol.UINode {
	if depth >= protocol.MaxUINodeDepth {
		return protocol.Text("[render limit exceeded]")
	}

	style := node.Style
	node.Style = nil
	switch node.Component {
	case "screen":
		node.Style = sanitizeContainerStyle(style)
		node.Children = sanitizeRemoteChildren(node.Children, depth+1)
	case "header", "status":
		node.Text = sanitizeChromeText(node.Text)
		node.Children = nil
	case "text":
		node.Text = sanitizeMultilineText(node.Text)
		node.Children = nil
	case "markdown":
		node.Text = sanitizeMarkdownText(node.Text)
		node.Children = nil
	case "panel":
		node.Style = sanitizeContainerStyle(style)
		node.Title = sanitizeChromeText(node.Title)
		node.Children = sanitizeRemoteChildren(node.Children, depth+1)
	case "menu", "list", "dynamic_list":
		node.ID = sanitizeChromeText(node.ID)
		node.Items = sanitizeRemoteItems(node.Items)
		node.Children = nil
	case "input", "textarea":
		node.ID = sanitizeChromeText(node.ID)
		node.Placeholder = sanitizeChromeText(node.Placeholder)
		node.Value = sanitizeMultilineText(node.Value)
		node.Children = nil
	case "button", "checkbox":
		node.ID = sanitizeChromeText(node.ID)
		node.Text = sanitizeChromeText(node.Text)
		node.Action = sanitizeChromeText(node.Action)
		node.Children = nil
	case "grid":
		node.ID = sanitizeChromeText(node.ID)
		node.Rows = sanitizeRemoteGrid(node.Rows)
		node.Children = nil
	case "log":
		node.Style = sanitizeContainerStyle(style)
		node.Children = sanitizeRemoteChildren(node.Children, depth+1)
	default:
		return protocol.Text("[unsupported component]")
	}

	return node
}

func sanitizeContainerStyle(style *protocol.UIStyle) *protocol.UIStyle {
	if style == nil || style.Background == nil {
		return nil
	}
	background, ok := sanitizeBackground(style.Background)
	if !ok {
		return nil
	}
	return &protocol.UIStyle{Background: background}
}

func sanitizeBackground(background *protocol.UIBackground) (*protocol.UIBackground, bool) {
	result := &protocol.UIBackground{
		Kind:      sanitizeChromeText(background.Kind),
		Direction: sanitizeChromeText(background.Direction),
		Color:     sanitizeChromeText(background.Color),
		From:      sanitizeChromeText(background.From),
		To:        sanitizeChromeText(background.To),
	}
	if len(background.Stops) > 0 {
		limit := minInt(len(background.Stops), protocol.MaxUIGradientStops)
		result.Stops = make([]protocol.UIGradientStop, 0, limit)
		for index := 0; index < limit; index++ {
			stop := background.Stops[index]
			color := sanitizeChromeText(stop.Color)
			if !isValidHexColor(color) {
				continue
			}
			result.Stops = append(result.Stops, protocol.UIGradientStop{
				At:    clampFloat(stop.At, 0, 1),
				Color: color,
			})
		}
	}
	if _, ok := resolveContainerBackground(&protocol.UIStyle{Background: result}); !ok {
		return nil, false
	}
	return result, true
}

func sanitizeRemoteChildren(children []protocol.UINode, depth int) []protocol.UINode {
	limit := minInt(len(children), protocol.MaxUIChildren)
	result := make([]protocol.UINode, 0, limit)
	for index := 0; index < limit; index++ {
		result = append(result, sanitizeRemoteNode(children[index], depth))
	}
	if len(children) > limit {
		result = append(result, protocol.Text("[content truncated]"))
	}
	return result
}

func sanitizeRemoteItems(items []protocol.Item) []protocol.Item {
	limit := minInt(len(items), protocol.MaxUIItems)
	result := make([]protocol.Item, 0, limit)
	for index := 0; index < limit; index++ {
		item := items[index]
		item.Label = sanitizeChromeText(item.Label)
		item.Action = sanitizeChromeText(item.Action)
		result = append(result, item)
	}
	if len(items) > limit {
		result = append(result, protocol.Item{Label: "[more choices truncated]"})
	}
	return result
}

func sanitizeRemoteGrid(rows [][]string) [][]string {
	rowLimit := minInt(len(rows), protocol.MaxGridRows)
	result := make([][]string, 0, rowLimit)
	for rowIndex := 0; rowIndex < rowLimit; rowIndex++ {
		row := rows[rowIndex]
		cellLimit := minInt(len(row), protocol.MaxGridCols)
		cells := make([]string, 0, cellLimit)
		for cellIndex := 0; cellIndex < cellLimit; cellIndex++ {
			cells = append(cells, sanitizeChromeText(row[cellIndex]))
		}
		result = append(result, cells)
	}
	if len(rows) > rowLimit {
		result = append(result, []string{"[grid truncated]"})
	}
	return result
}
