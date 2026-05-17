package client

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type trustPromptModel struct {
	summary  trustSummary
	selected int
	accepted bool
	decided  bool
	width    int
	height   int
}

func runFirstConnectTrustTUI(input io.Reader, output io.Writer, summary trustSummary) error {
	model := newTrustPromptModel(summary)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	result, ok := finalModel.(trustPromptModel)
	if !ok || !result.accepted {
		return fmt.Errorf("station identity was not pinned")
	}
	return nil
}

func newTrustPromptModel(summary trustSummary) trustPromptModel {
	return trustPromptModel{
		summary: summary,
		width:   88,
		height:  28,
	}
}

func (m trustPromptModel) Init() tea.Cmd {
	return nil
}

func (m trustPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q", "n":
			m.accepted = false
			m.decided = true
			return m, tea.Quit
		case "y":
			m.accepted = true
			m.decided = true
			return m, tea.Quit
		case "tab", "right", "left":
			if m.selected == 0 {
				m.selected = 1
			} else {
				m.selected = 0
			}
		case "enter":
			m.accepted = m.selected == 0
			m.decided = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m trustPromptModel) View() string {
	width := minInt(maxInt(m.width-8, 64), 96)
	panelWidth := maxInt(width-4, 40)
	appBackground := lipgloss.Color("#0B0F14")
	panelBackground := lipgloss.Color("#151A20")

	appStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Foreground(lipgloss.Color("#E8F0F2")).
		Background(appBackground)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#9BE9A8")).
		Background(panelBackground)
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58A6FF")).
		Background(panelBackground)
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B949E")).
		Background(panelBackground)
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8F0F2")).
		Background(panelBackground)
	noticeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E3B341")).
		Background(panelBackground)
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3A4A5A")).
		Background(panelBackground)
	focusedButton := buttonStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#0B0F14")).
		Background(lipgloss.Color("#9BE9A8")).
		BorderForeground(lipgloss.Color("#9BE9A8"))
	cancelButton := buttonStyle.Copy().
		Foreground(lipgloss.Color("#FF7B72"))
	focusedCancel := cancelButton.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#0B0F14")).
		Background(lipgloss.Color("#FF7B72")).
		BorderForeground(lipgloss.Color("#FF7B72"))

	rows := []string{
		titleStyle.Render("PhosphorNet First Connection Trust"),
		labelStyle.Render("Trusted local client screen"),
		"",
		sectionStyle.Render("Transport"),
		trustFact(labelStyle, valueStyle, "Encryption", m.summary.Transport, panelWidth),
		trustFact(labelStyle, valueStyle, "Certificate", m.summary.Certificate, panelWidth),
		trustFact(labelStyle, valueStyle, "Hostname", m.summary.HostnameVerification, panelWidth),
		"",
		sectionStyle.Render("Station Identity"),
		trustFact(labelStyle, valueStyle, "Name", m.summary.StationName, panelWidth),
		trustFact(labelStyle, valueStyle, "Address", m.summary.Address, panelWidth),
		trustFact(labelStyle, valueStyle, "Ed25519", m.summary.StationIdentity, panelWidth),
		trustFact(labelStyle, valueStyle, "Public key", m.summary.StationPublicKey, panelWidth),
		"",
		noticeStyle.Width(panelWidth).Render("Transport security, certificate status, and pinned station identity are separate facts. Pin this Ed25519 station identity only if this is the station you meant to visit."),
		"",
	}

	accept := buttonStyle.Render("Trust + Pin")
	cancel := cancelButton.Render("Cancel")
	if m.selected == 0 {
		accept = focusedButton.Render("Trust + Pin")
	} else {
		cancel = focusedCancel.Render("Cancel")
	}
	rows = append(rows,
		lipgloss.JoinHorizontal(lipgloss.Top, accept, "  ", cancel),
		labelStyle.Render("enter select  tab/left/right switch  y trust  n/esc cancel"),
	)

	panel := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#58A6FF")).
		BorderBackground(panelBackground).
		Background(panelBackground).
		Render(strings.Join(rows, "\n"))
	return appStyle.Render(panel)
}

func trustFact(labelStyle, valueStyle lipgloss.Style, label, value string, width int) string {
	labelColumn := labelStyle.Width(13).Render(label + ":")
	valueColumn := valueStyle.Width(maxInt(width-14, 20)).Render(value)
	return lipgloss.JoinHorizontal(lipgloss.Top, labelColumn, valueColumn)
}
