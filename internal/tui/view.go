package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	hintsBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("2")).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	focusedPaneStyle = paneStyle.
				BorderForeground(lipgloss.Color("62"))

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62"))

	itemStyle = lipgloss.NewStyle()

	statusOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	statusDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	lineNumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	sepStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m *Model) View() string {
	if !m.ready {
		return "cargando...\n"
	}

	hints := hintsBarStyle.Render(" hels dashboard — click o j/k elige servicio · Tab cambia de panel · click/rueda en logs para scrollear · q sale ")

	list := m.renderList()
	logs := m.renderLogs()

	body := lipgloss.JoinHorizontal(lipgloss.Top, list, logs)

	return lipgloss.JoinVertical(lipgloss.Left, body, hints)
}

func (m *Model) renderList() string {
	content := "SERVICIOS\n\n"

	if m.err != nil {
		content += errStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n"
	} else if len(m.containers) == 0 {
		content += statusDimStyle.Render("(sin contenedores corriendo)") + "\n"
	}

	for i, c := range m.containers {
		dot := statusOKStyle.Render("●")
		line := fmt.Sprintf("%s %s", dot, c.Name)
		if i == m.cursor {
			content += selectedItemStyle.Render(padRight(line, listPaneWidth)) + "\n"
		} else {
			content += itemStyle.Render(line) + "\n"
		}
	}

	style := paneStyle
	if m.focus == focusList {
		style = focusedPaneStyle
	}

	return style.
		Width(listPaneWidth).
		Height(m.viewport.Height + paneTitleLines).
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

	style := paneStyle
	if m.focus == focusLogs {
		style = focusedPaneStyle
	}

	return style.
		Width(m.viewport.Width).
		Height(m.viewport.Height + paneTitleLines).
		Render(content)
}

// padRight rellena s con espacios hasta n runas, para que el resaltado de
// selección cubra todo el ancho de la fila y no solo el texto.
func padRight(s string, n int) string {
	pad := n - len([]rune(s))
	if pad <= 0 {
		return s
	}
	for i := 0; i < pad; i++ {
		s += " "
	}
	return s
}

// numberedLog arma el texto de los logs con número de línea (1-based) y un
// separador, coloreados, antes de que se envuelva (wrap) al ancho del panel.
func numberedLog(lines []string) string {
	out := make([]string, len(lines))
	sep := sepStyle.Render("│")
	for i, l := range lines {
		num := lineNumStyle.Render(fmt.Sprintf("%4d", i+1))
		out[i] = num + " " + sep + " " + l
	}
	return strings.Join(out, "\n")
}
