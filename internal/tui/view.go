package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("4")).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62"))

	itemStyle = lipgloss.NewStyle()

	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m *Model) View() string {
	if !m.ready {
		return "cargando...\n"
	}

	header := headerStyle.Render(" hels dashboard — j/k o ↑/↓ para elegir servicio, q para salir ")

	list := m.renderList()
	logs := m.renderLogs()

	body := lipgloss.JoinHorizontal(lipgloss.Top, list, logs)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *Model) renderList() string {
	content := "SERVICIOS\n\n"

	if m.err != nil {
		content += errStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n"
	} else if len(m.containers) == 0 {
		content += statusStyle.Render("(sin contenedores corriendo)") + "\n"
	}

	for i, c := range m.containers {
		line := fmt.Sprintf("%s\n%s", c.Name, statusStyle.Render(c.Status))
		if i == m.cursor {
			content += selectedItemStyle.Render(line) + "\n"
		} else {
			content += itemStyle.Render(line) + "\n"
		}
	}

	return paneStyle.
		Width(listPaneWidth).
		Height(m.viewport.Height).
		Render(content)
}

func (m *Model) renderLogs() string {
	title := "LOGS"
	if m.selectedID != "" && len(m.containers) > 0 {
		title = fmt.Sprintf("LOGS — %s", m.containers[m.cursor].Name)
	}

	content := title + "\n\n" + m.viewport.View()
	if m.logErr != nil {
		content += "\n" + errStyle.Render(fmt.Sprintf("stream de logs terminó: %v", m.logErr))
	}

	return paneStyle.
		Width(m.viewport.Width).
		Height(m.viewport.Height).
		Render(content)
}
