package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	tailLines      = 200
	maxLogLines    = 2000
	refreshEvery   = 3 * time.Second
	listPaneWidth  = 28
	headerHeight   = 3
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
		vpWidth := msg.Width - listPaneWidth - 3
		vpHeight := msg.Height - headerHeight
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
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
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
