package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	outerHeaderLines = 1 // la barra azul de arriba
	paneBorderLines  = 2 // borde de cada panel: arriba + abajo
	paneTitleLines   = 2 // "TÍTULO" + línea en blanco, dentro de cada panel
	paneChrome       = 4 // borde (1+1) + padding horizontal (1+1) de cada panel

	listPaneWidth      = 28
	listPaneOuterWidth = listPaneWidth + paneChrome
	listItemsTopRow    = outerHeaderLines + 1 + paneTitleLines // + 1 = borde-top
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

type logLineMsg struct {
	gen  int
	line string
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
		vpHeight := msg.Height - outerHeaderLines - paneBorderLines - paneTitleLines
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

	case logLineMsg:
		if msg.gen != m.logGen {
			return m, nil
		}
		m.logLines = append(m.logLines, msg.line)
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		atBottom := m.viewport.AtBottom()
		m.viewport.SetContent(joinLines(m.logLines))
		if atBottom {
			m.viewport.GotoBottom()
		}
		return m, waitForLogLine(m.stream, msg.gen)

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

	stream, err := startLogStream(target.ID, tailLines)
	if err != nil {
		m.logErr = err
		return nil
	}
	m.stream = stream

	return waitForLogLine(stream, gen)
}

func (m *Model) stopStream() {
	if m.stream != nil {
		m.stream.Stop()
		m.stream = nil
	}
	m.selectedID = ""
}

// waitForLogLine devuelve un tea.Cmd que espera la próxima línea del stream
// dado. Cada línea recibida vuelve a encadenar esta misma espera (patrón
// estándar de bubbletea para consumir un canal sin bloquear el Update loop).
func waitForLogLine(stream *logStream, gen int) tea.Cmd {
	if stream == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-stream.Lines
		if !ok {
			err := <-stream.Done
			return logStreamErrMsg{gen: gen, err: err}
		}
		return logLineMsg{gen: gen, line: line}
	}
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
