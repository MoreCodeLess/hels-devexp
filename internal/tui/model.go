package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const (
	tailLines    = 200
	maxLogLines  = 2000
	refreshEvery = 3 * time.Second

	// Layout: estas constantes describen el presupuesto vertical/horizontal
	// exacto que consume cada elemento de chrome (todo lo que no es
	// contenido real), para que el alto total nunca exceda la terminal
	// (lipgloss NO trunca contenido más alto que el Height() pedido, así que
	// cualquier desfase acá desborda el alt-screen). listItemsTopRow y
	// listPaneOuterWidth también se usan para traducir un click de mouse a
	// fila/columna (hitTestList más abajo) — si se cambia el layout en un
	// lado hay que actualizar el otro.
	//
	// La barra de hints va ABAJO de todo, así que no suma nada antes del
	// panel de lista/logs (listItemsTopRow arranca directo en el borde).
	outerBarLines   = 1 // la barra de hints, al fondo
	paneBorderLines = 2 // borde de cada panel: arriba + abajo
	paneTitleLines  = 2 // "TÍTULO" + línea en blanco, dentro de cada panel
	logStatusLines  = 1 // fila de estado del panel de logs (en blanco si no hay error)
	paneChrome      = 4 // borde (1+1) + padding horizontal (1+1) de cada panel

	listPaneWidth      = 28
	listPaneOuterWidth = listPaneWidth + paneChrome
	listItemsTopRow    = 1 + paneTitleLines // 1 = borde-top del panel
)

// focusArea indica qué panel recibe el teclado y el scroll en este momento.
type focusArea int

const (
	focusList focusArea = iota
	focusLogs
)

type containersLoadedMsg struct {
	containers []Container
	err        error
}

type refreshTickMsg time.Time

// logLinesMsg trae un lote de líneas ya listas del stream, no una por
// mensaje — ver waitForLogLines para el porqué.
type logLinesMsg struct {
	gen   int
	lines []string
}

type logStreamErrMsg struct {
	gen int
	err error
}

// Model es el estado del dashboard interactivo de hels.
type Model struct {
	program *tea.Program

	containers []Container
	cursor     int
	selectedID string
	focus      focusArea

	logLines []string
	logGen   int
	stream   *logStream
	logErr   error

	viewport viewport.Model
	ready    bool
	err      error

	width, height int
}

// New crea el modelo inicial del dashboard.
func New() *Model {
	return &Model{}
}

// AttachProgram le da al modelo una referencia al *tea.Program que lo corre,
// para que los goroutines de streaming de logs puedan mandarle mensajes.
// Hay que llamarlo antes de p.Run().
func (m *Model) AttachProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(loadContainersCmd(), tickCmd())
}

func loadContainersCmd() tea.Cmd {
	return func() tea.Msg {
		cs, err := listContainers()
		return containersLoadedMsg{containers: cs, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshEvery, func(t time.Time) tea.Msg { return refreshTickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		vpWidth := msg.Width - listPaneWidth - 2*paneChrome
		vpHeight := msg.Height - outerBarLines - paneBorderLines - paneTitleLines - logStatusLines
		if vpWidth < 1 {
			vpWidth = 1
		}
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}
		// El ancho pudo haber cambiado (resize de la terminal): hay que
		// re-envolver todo el buffer de logs ya acumulado a la medida nueva,
		// si no las líneas viejas quedan envueltas al ancho anterior y
		// pueden volver a desbordar.
		m.refreshLogViewport()
		return m, nil

	case containersLoadedMsg:
		m.err = msg.err
		m.containers = msg.containers
		if len(m.containers) == 0 {
			m.cursor = 0
			m.selectedID = ""
			m.stopStream()
			return m, nil
		}
		if m.cursor >= len(m.containers) {
			m.cursor = len(m.containers) - 1
		}
		return m, m.ensureStreamForCursor()

	case refreshTickMsg:
		return m, tea.Batch(loadContainersCmd(), tickCmd())

	case logLinesMsg:
		if msg.gen != m.logGen {
			return m, nil
		}
		m.logLines = append(m.logLines, msg.lines...)
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		m.refreshLogViewport()
		return m, waitForLogLines(m.stream, msg.gen)

	case logStreamErrMsg:
		if msg.gen != m.logGen {
			return m, nil
		}
		m.logErr = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.stopStream()
			return m, tea.Quit
		case "tab":
			m.toggleFocus()
			return m, nil
		}

		if m.focus == focusList {
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					return m, m.ensureStreamForCursor()
				}
			case "down", "j":
				if m.cursor < len(m.containers)-1 {
					m.cursor++
					return m, m.ensureStreamForCursor()
				}
			}
			return m, nil
		}
		// focusLogs: cualquier tecla (arriba/abajo/j/k, pgup/pgdn, home/end,
		// ctrl+u/d, ...) se la pasamos directo al viewport para que scrollee.

	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			if idx, ok := m.hitTestList(msg.X, msg.Y); ok {
				m.focus = focusList
				m.cursor = idx
				return m, m.ensureStreamForCursor()
			}
			m.focus = focusLogs
			return m, nil

		case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
			// la rueda scrollea los logs según dónde esté el mouse, sin
			// depender de qué panel tenga el foco del teclado ni tocar la
			// lista de servicios.
			if msg.X < listPaneOuterWidth {
				return m, nil
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if m.focus != focusLogs {
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) toggleFocus() {
	if m.focus == focusList {
		m.focus = focusLogs
	} else {
		m.focus = focusList
	}
}

// hitTestList traduce coordenadas de mouse a un índice de la lista de
// servicios. Devuelve ok=false si el click cayó fuera del panel de lista o
// sobre una fila sin servicio.
func (m *Model) hitTestList(x, y int) (int, bool) {
	if x < 0 || x >= listPaneOuterWidth {
		return 0, false
	}
	idx := y - listItemsTopRow
	if idx < 0 || idx >= len(m.containers) {
		return 0, false
	}
	return idx, true
}

// ensureStreamForCursor arranca el stream de logs del contenedor bajo el
// cursor si todavía no es el seleccionado.
func (m *Model) ensureStreamForCursor() tea.Cmd {
	if len(m.containers) == 0 {
		return nil
	}
	target := m.containers[m.cursor]
	if target.ID == m.selectedID {
		return nil
	}

	m.stopStream()
	m.selectedID = target.ID
	m.logLines = nil
	m.logErr = nil
	m.logGen++
	gen := m.logGen
	m.refreshLogViewport()

	stream, err := startLogStream(target.ID, tailLines)
	if err != nil {
		m.logErr = err
		return nil
	}
	m.stream = stream

	return waitForLogLines(stream, gen)
}

func (m *Model) stopStream() {
	if m.stream != nil {
		m.stream.Stop()
		m.stream = nil
	}
	m.selectedID = ""
}

// waitForLogLines devuelve un tea.Cmd que espera al menos una línea nueva del
// stream y de paso agrupa (drena, sin bloquear) cualquier otra línea que ya
// esté esperando en el canal. Esto importa mucho para un contenedor con
// historial largo (ej. sshd con cientos de conexiones logueadas): al
// seleccionarlo, "docker logs --tail N" entrega ese historial casi de
// golpe, y sin agrupar cada línea dispara su propio ciclo completo de
// Update+render (con su propio re-wrap de todo el buffer) — una ráfaga de
// cientos de mensajes en milisegundos que satura el render y rompe la
// vista. Agrupando, esa misma ráfaga se procesa en uno o pocos renders.
func waitForLogLines(stream *logStream, gen int) tea.Cmd {
	if stream == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-stream.Lines
		if !ok {
			err := <-stream.Done
			return logStreamErrMsg{gen: gen, err: err}
		}

		lines := []string{line}
	drain:
		for {
			select {
			case l, ok := <-stream.Lines:
				if !ok {
					break drain
				}
				lines = append(lines, l)
			default:
				break drain
			}
		}

		return logLinesMsg{gen: gen, lines: lines}
	}
}

// refreshLogViewport reconstruye el contenido visible del panel de logs a
// partir de m.logLines.
//
// El panel de logs es un "portal" de tamaño fijo: su alto (m.viewport.Height)
// nunca cambia por el contenido, sin importar cuántas líneas físicas termine
// ocupando una entrada larga. Para lograr eso de forma confiable, PARTIMOS
// cada línea lógica nosotros mismos en pedazos de a lo sumo m.viewport.Width
// celdas visibles (hardWrapLine, usando ansi.Cut — el mismo primitivo que usa
// bubbles/viewport internamente para su propio recorte) y le damos al
// viewport la lista COMPLETA de pedazos ya partidos. El viewport se encarga
// solo de la ventana vertical (YOffset + Height): sin importar cuántos
// pedazos totales haya, siempre muestra como mucho Height filas — por eso el
// tamaño de una línea no puede afectar el alto del panel, solo cuánto hay
// para scrollear dentro de él.
//
// (Antes se dejaba que el viewport cortara cada línea larga por su cuenta
// sin wrapear. Eso rompía la invariante de todos modos: el propio View() del
// viewport vuelve a pasar el contenido por un Height/MaxHeight de lipgloss
// después de cortar, y esa segunda pasada podía terminar generando una fila
// de más para una única línea muy larga.)
func (m *Model) refreshLogViewport() {
	if !m.ready {
		// Todavía no llegó el primer tea.WindowSizeMsg (viewport sin
		// inicializar); no hay nada que dibujar todavía.
		return
	}

	width := m.viewport.Width
	if width < 1 {
		width = 1
	}

	var display []string
	for i, l := range m.logLines {
		display = append(display, hardWrapLine(numberedLine(i, l), width)...)
	}

	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(strings.Join(display, "\n"))
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// hardWrapLine parte line en pedazos de a lo sumo width celdas visibles,
// usando ansi.Cut (el mismo primitivo que bubbles/viewport usa para su
// propio recorte horizontal), para que no haya ninguna discrepancia de
// medición de ancho entre cómo partimos acá y cómo el viewport vuelve a
// medir después.
func hardWrapLine(line string, width int) []string {
	total := ansi.StringWidth(line)
	if total <= width {
		return []string{line}
	}

	var out []string
	for start := 0; start < total; start += width {
		end := start + width
		if end > total {
			end = total
		}
		out = append(out, ansi.Cut(line, start, end))
	}
	return out
}
