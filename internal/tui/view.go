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
	statusBadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	statusDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m *Model) View() string {
	if !m.ready {
		return "cargando...\n"
	}

	hints := hintsBarStyle.Render(" hels dashboard — click o j/k elige · ←/→ cambia categoría · Tab cambia de panel · r reinicia el proceso · rueda/pgup/pgdn scrollea · q sale ")

	list := m.renderList()
	logs := m.renderLogs()

	body := lipgloss.JoinHorizontal(lipgloss.Top, list, logs)

	return lipgloss.JoinVertical(lipgloss.Left, body, hints)
}

// renderList arma el panel izquierdo: título, menú de categorías clickeable,
// y una VENTANA de tamaño fijo sobre visibleItems() (con scroll si no
// entran todos). Es un "portal" igual que el panel de logs (ver
// refreshLogViewport): el total de filas de contenido tiene que dar
// SIEMPRE exactamente m.viewport.Height+paneTitleLines+logStatusLines, sin
// importar cuántos ítems haya — si no, lipgloss no trunca lo que sobra (no
// tiene forma de hacerlo) y el panel entero se desborda, rompiendo la vista
// con muchos ítems. Por eso el mensaje de error/lista-vacía también cuenta
// contra ese presupuesto en vez de sumarse aparte.
func (m *Model) renderList() string {
	content := "SERVICIOS\n\n"
	content += m.renderCategoryTabs() + "\n"

	visible := m.visibleItems()
	budget := m.viewport.Height
	var body []string

	switch {
	case m.err != nil:
		body = append(body, errStyle.Render(fmt.Sprintf("error: %v", m.err)))
		budget--
	case len(visible) == 0:
		body = append(body, statusDimStyle.Render("(nada corriendo todavía)"))
		budget--
	}

	maxVisible := budget / 2
	if maxVisible < 0 {
		maxVisible = 0
	}
	start := m.listScroll
	if start > len(visible) {
		start = len(visible)
	}
	end := start + maxVisible
	if end > len(visible) {
		end = len(visible)
	}

	// Cada ítem ocupa SIEMPRE 2 filas (nombre + estado), esté seleccionado o
	// no — hitTestList asume ese alto fijo para traducir un click a un
	// índice de la lista (ver el comentario ahí).
	for i := start; i < end; i++ {
		it := visible[i]
		dotStyle := statusOKStyle
		if !it.ok {
			dotStyle = statusBadStyle
		}
		dot := dotStyle.Render(itemIcon(it.kind))

		nameLine := fmt.Sprintf("%s %s", dot, it.name)
		statusLine := "  " + it.status

		// El ancho se fuerza ACÁ, línea por línea, con el propio .Width() de
		// lipgloss (mide con el mismo criterio ansi-aware que usa el resto
		// del código, a diferencia del padRight manual que había antes) — y
		// por eso el panel entero, más abajo, NO vuelve a pedir .Width(): es
		// la misma combinación "Width() redundante + Padding()" que ya
		// habíamos visto romper el panel de logs (ver renderLogs), y acá
		// pasaba con la fila seleccionada en cuanto la lista dejó de crecer
		// sin límite y empezó a exigir un alto exacto.
		if i == m.cursor {
			body = append(body, selectedItemStyle.Width(listPaneWidth).Render(nameLine))
			body = append(body, selectedItemStyle.Width(listPaneWidth).Render(statusLine))
		} else {
			body = append(body, itemStyle.Width(listPaneWidth).Render(nameLine))
			body = append(body, statusDimStyle.Width(listPaneWidth).Render(statusLine))
		}
	}

	for len(body) < m.viewport.Height {
		body = append(body, "")
	}
	content += strings.Join(body, "\n")

	style := paneStyle
	if m.focus == focusList {
		style = focusedPaneStyle
	}

	return style.
		Height(m.viewport.Height + paneTitleLines + logStatusLines).
		Render(content)
}

// renderCategoryTabs arma la fila de pestañas clickeables ("[*] [▶] [●] ..."
// ) — comparte layout exacto con hitTestCategoryTabs para que el click
// siempre caiga sobre lo que se está viendo.
func (m *Model) renderCategoryTabs() string {
	tabs := m.categoryTabsLayout()
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		if t.kind == m.categoryFilter {
			parts[i] = selectedItemStyle.Render(t.label)
		} else {
			parts[i] = statusDimStyle.Render(t.label)
		}
	}
	return strings.Join(parts, " ")
}

func (m *Model) renderLogs() string {
	title := "LOGS"
	if visible := m.visibleItems(); len(visible) > 0 && m.cursor < len(visible) {
		kind := "proceso"
		switch visible[m.cursor].kind {
		case kindContainer:
			kind = "contenedor"
		case kindLambda:
			kind = "lambda"
		case kindQueue:
			kind = "cola"
		case kindTopic:
			kind = "tópico"
		}
		title = fmt.Sprintf("LOGS — %s (%s)", visible[m.cursor].name, kind)
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
