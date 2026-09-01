package tui

import (
	"fmt"

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
	statusBadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	statusDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m *Model) View() string {
	if !m.ready {
		return "cargando...\n"
	}

	hints := hintsBarStyle.Render(" hels dashboard — click o j/k elige · Tab cambia de panel · r reinicia el proceso · rueda/pgup/pgdn scrollea vertical · h/l scrollea horizontal · q sale ")

	list := m.renderList()
	logs := m.renderLogs()

	body := lipgloss.JoinHorizontal(lipgloss.Top, list, logs)

	return lipgloss.JoinVertical(lipgloss.Left, body, hints)
}

func (m *Model) renderList() string {
	content := "SERVICIOS\n\n"

	if m.err != nil {
		content += errStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n"
	} else if len(m.items) == 0 {
		content += statusDimStyle.Render("(nada corriendo todavía)") + "\n"
	}

	// Cada ítem ocupa SIEMPRE 2 filas (nombre + estado), esté seleccionado o
	// no — hitTestList asume ese ancho fijo para traducir un click a un
	// índice de la lista (ver el comentario ahí).
	for i, it := range m.items {
		icon := "▶" // proceso local (declarado en hels.yaml)
		if it.kind == kindContainer {
			icon = "●" // contenedor de infra (Docker)
		}
		dotStyle := statusOKStyle
		if !it.ok {
			dotStyle = statusBadStyle
		}
		dot := dotStyle.Render(icon)

		nameLine := fmt.Sprintf("%s %s", dot, it.name)
		statusLine := "  " + it.status

		if i == m.cursor {
			content += selectedItemStyle.Render(padRight(nameLine, listPaneWidth)) + "\n"
			content += selectedItemStyle.Render(padRight(statusLine, listPaneWidth)) + "\n"
		} else {
			content += itemStyle.Render(nameLine) + "\n"
			content += statusDimStyle.Render(statusLine) + "\n"
		}
	}

	style := paneStyle
	if m.focus == focusList {
		style = focusedPaneStyle
	}

	return style.
		Width(listPaneWidth).
		Height(m.viewport.Height + paneTitleLines + logStatusLines).
		Render(content)
}

func (m *Model) renderLogs() string {
	title := "LOGS"
	if len(m.items) > 0 && m.cursor < len(m.items) {
		kind := "proceso"
		if m.items[m.cursor].kind == kindContainer {
			kind = "contenedor"
		}
		title = fmt.Sprintf("LOGS — %s (%s)", m.items[m.cursor].name, kind)
	}

	// Reservamos siempre 1 fila de estado (en blanco si no hay error), para
	// que el contenido tenga la MISMA cantidad de filas sin importar si hay
	// error o no — el panel nunca cambia de alto según el estado del stream.
	status := ""
	if err := m.currentLogErr(); err != nil {
		status = errStyle.Render(fmt.Sprintf("terminó: %v", err))
	}

	content := title + "\n\n" + m.viewport.View() + "\n" + status

	style := paneStyle
	if m.focus == focusLogs {
		style = focusedPaneStyle
	}

	// Ojo: a propósito NO se llama .Width() acá. m.viewport.View() ya rellena
	// cada línea a exactamente m.viewport.Width por su cuenta (su propio
	// Style interno hace lo mismo Width/Height que hacemos nosotros afuera),
	// así que el contenido YA es uniforme en ancho. Pedirle a este panel
	// EXTERNO que además fuerce ese mismo ancho es redundante — y en la
	// práctica, combinado con el padding(0,1) de paneStyle, dispara un bug
	// real de lipgloss donde líneas que ya miden justo el ancho pedido
	// terminan partidas en 2 filas (confirmado con contenido con y sin
	// color ANSI). Sin el .Width() acá, lipgloss igual alinea todo al ancho
	// más largo del contenido (que ya es m.viewport.Width), pero sin pasar
	// por ese camino roto.
	return style.
		Height(m.viewport.Height + paneTitleLines + logStatusLines).
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

// numberedLine arma una entrada de log con su número de línea (1-based) y un
// separador. i es el índice 0-based dentro del buffer que se esté mostrando.
//
// A propósito es texto plano, sin ningún color ANSI. hardWrapLine corta esto
// con ansi.Cut para armar el wrap manual del panel de logs, y en la práctica
// el corte de contenido ESTILADO (con códigos de color) se desalinea del
// contenido real — el número/separador terminaba "empujando" el texto a
// otra fila en vez de compartir la misma. Con texto plano el corte es
// exacto en todos los casos probados.
func numberedLine(i int, line string) string {
	return fmt.Sprintf("%4d │ %s", i+1, line)
}
