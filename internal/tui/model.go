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

	listPaneWidth      = 30
	listPaneOuterWidth = listPaneWidth + paneChrome
	listItemsTopRow    = 1 + paneTitleLines // 1 = borde-top del panel
)

// focusArea indica qué panel recibe el teclado y el scroll en este momento.
type focusArea int

const (
	focusList focusArea = iota
	focusLogs
)

// itemKind distingue las dos fuentes de "servicios" que se ven en la lista.
type itemKind int

const (
	kindProcess itemKind = iota
	kindContainer
)

// listItem es una fila de la lista: o un proceso local (declarado en
// hels.yaml, corrido por hels) o un contenedor Docker (infra: floci, o
// cualquier otro contenedor visible en la máquina).
type listItem struct {
	kind   itemKind
	key    string // "proc:<nombre>" o el ID del contenedor
	name   string
	status string
	ok     bool // true = corriendo/sano, false = detenido/error (para el color del punto)
}

// procEntry es un proceso local en ejecución (o ya terminado), con su
// buffer de líneas propio y persistente — a diferencia del log de un
// contenedor, sigue acumulando salida en segundo plano sin importar qué
// esté seleccionado en la lista, igual que en mprocs.
type procEntry struct {
	spec    ProcessSpec
	handle  *procHandle
	lines   []string
	running bool
	exitErr error
	gen     int
}

type containersLoadedMsg struct {
	containers []Container
	err        error
}

type refreshTickMsg time.Time

// logLinesMsg trae un lote de líneas ya listas del stream de un CONTENEDOR,
// no una por mensaje — ver waitForLogLines para el porqué.
type logLinesMsg struct {
	gen   int
	lines []string
}

type logStreamErrMsg struct {
	gen int
	err error
}

// procLinesMsg/procExitMsg son el equivalente para procesos locales: se
// procesan siempre (no solo cuando el proceso está seleccionado), porque
// siguen corriendo en segundo plano.
type procLinesMsg struct {
	name  string
	gen   int
	lines []string
}

type procExitMsg struct {
	name string
	gen  int
	err  error
}

// Model es el estado del dashboard interactivo de hels.
type Model struct {
	program *tea.Program

	processSpecs []ProcessSpec
	processes    []*procEntry

	containers []Container
	items      []listItem
	cursor     int
	selectedKey string
	focus       focusArea

	// Estado del stream de logs de un CONTENEDOR seleccionado (on-demand: se
	// arranca al seleccionar, se para al deseleccionar). Los procesos no
	// usan esto — su buffer vive en procEntry.lines.
	containerLogLines []string
	containerLogGen   int
	containerStream   *logStream
	containerLogErr   error

	viewport viewport.Model
	ready    bool
	err      error

	width, height int
}

// New crea el modelo inicial del dashboard. specs son los procesos locales
// declarados en hels.yaml (processes.*) — puede ser nil/vacío si no hay
// hels.yaml o no declara ninguno; el dashboard sigue funcionando mostrando
// solo la infraestructura Docker.
func New(specs []ProcessSpec) *Model {
	return &Model{processSpecs: specs}
}

// AttachProgram le da al modelo una referencia al *tea.Program que lo corre,
// para que los goroutines de streaming de logs puedan mandarle mensajes.
// Hay que llamarlo antes de p.Run().
func (m *Model) AttachProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadContainersCmd(), tickCmd()}
	cmds = append(cmds, m.startAllProcesses()...)
	return tea.Batch(cmds...)
}

// startAllProcesses arranca, de una, todos los procesos declarados en
// hels.yaml — igual que mprocs, todos corren desde que se abre el dashboard,
// no hace falta seleccionarlos primero.
func (m *Model) startAllProcesses() []tea.Cmd {
	var cmds []tea.Cmd
	for _, spec := range m.processSpecs {
		pe := &procEntry{spec: spec, gen: 1}
		handle, err := startProcess(spec.Cmd, spec.Dir)
		if err != nil {
			pe.exitErr = err
		} else {
			pe.handle = handle
			pe.running = true
			cmds = append(cmds, waitForProcLines(spec.Name, handle, pe.gen))
		}
		m.processes = append(m.processes, pe)
	}
	m.rebuildItems()
	return cmds
}

func (m *Model) findProcess(name string) *procEntry {
	for _, pe := range m.processes {
		if pe.spec.Name == name {
			return pe
		}
	}
	return nil
}

// rebuildItems arma la lista combinada que se muestra en el panel
// izquierdo: procesos primero (front/back/gateway — lo que declaraste vos),
// contenedores después (infra: floci y demás).
func (m *Model) rebuildItems() {
	items := make([]listItem, 0, len(m.processes)+len(m.containers))

	for _, pe := range m.processes {
		status := "detenido"
		ok := false
		switch {
		case pe.running:
			status = "corriendo"
			ok = true
		case pe.exitErr != nil:
			status = "error: " + pe.exitErr.Error()
		}
		items = append(items, listItem{
			kind: kindProcess, key: "proc:" + pe.spec.Name, name: pe.spec.Name, status: status, ok: ok,
		})
	}

	for _, c := range m.containers {
		items = append(items, listItem{
			kind: kindContainer, key: c.ID, name: c.Name, status: c.Status, ok: true,
		})
	}

	m.items = items
	if m.cursor >= len(m.items) {
		m.cursor = maxInt(0, len(m.items)-1)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
		m.rebuildItems()
		return m, m.ensureSelection()

	case refreshTickMsg:
		return m, tea.Batch(loadContainersCmd(), tickCmd())

	case logLinesMsg:
		if msg.gen != m.containerLogGen {
			return m, nil
		}
		m.containerLogLines = append(m.containerLogLines, msg.lines...)
		if len(m.containerLogLines) > maxLogLines {
			m.containerLogLines = m.containerLogLines[len(m.containerLogLines)-maxLogLines:]
		}
		m.refreshLogViewport()
		return m, waitForLogLines(m.containerStream, msg.gen)

	case logStreamErrMsg:
		if msg.gen != m.containerLogGen {
			return m, nil
		}
		m.containerLogErr = msg.err
		return m, nil

	case procLinesMsg:
		pe := m.findProcess(msg.name)
		if pe == nil || pe.gen != msg.gen {
			return m, nil
		}
		pe.lines = append(pe.lines, msg.lines...)
		if len(pe.lines) > maxLogLines {
			pe.lines = pe.lines[len(pe.lines)-maxLogLines:]
		}
		if m.selectedKey == "proc:"+msg.name {
			m.refreshLogViewport()
		}
		return m, waitForProcLines(msg.name, pe.handle, msg.gen)

	case procExitMsg:
		pe := m.findProcess(msg.name)
		if pe == nil || pe.gen != msg.gen {
			return m, nil
		}
		pe.running = false
		pe.exitErr = msg.err
		m.rebuildItems()
		if m.selectedKey == "proc:"+msg.name {
			m.refreshLogViewport()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.stopEverything()
			return m, tea.Quit
		case "tab":
			m.toggleFocus()
			return m, nil
		case "r":
			return m, m.restartSelected()
		}

		if m.focus == focusList {
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					return m, m.ensureSelection()
				}
			case "down", "j":
				if m.cursor < len(m.items)-1 {
					m.cursor++
					return m, m.ensureSelection()
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
				return m, m.ensureSelection()
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
// sobre una fila sin servicio. Cada ítem ocupa 2 filas (nombre + estado, ver
// renderList en view.go), por eso se divide entre 2.
func (m *Model) hitTestList(x, y int) (int, bool) {
	if x < 0 || x >= listPaneOuterWidth {
		return 0, false
	}
	rowOffset := y - listItemsTopRow
	if rowOffset < 0 {
		return 0, false
	}
	idx := rowOffset / 2
	if idx >= len(m.items) {
		return 0, false
	}
	return idx, true
}

// ensureSelection arranca (si hace falta) el stream de logs del ítem bajo
// el cursor. Para un proceso no hay nada que "arrancar": ya viene corriendo
// solo desde Init, así que solo cambia qué buffer se muestra.
func (m *Model) ensureSelection() tea.Cmd {
	if len(m.items) == 0 {
		m.selectedKey = ""
		m.stopContainerStream()
		return nil
	}
	target := m.items[m.cursor]
	if target.key == m.selectedKey {
		return nil
	}

	m.stopContainerStream()
	m.selectedKey = target.key

	if target.kind == kindProcess {
		m.refreshLogViewport()
		return nil
	}

	m.containerLogLines = nil
	m.containerLogErr = nil
	m.containerLogGen++
	gen := m.containerLogGen

	stream, err := startLogStream(target.key, tailLines)
	if err != nil {
		m.containerLogErr = err
		m.refreshLogViewport()
		return nil
	}
	m.containerStream = stream
	m.refreshLogViewport()

	return waitForLogLines(stream, gen)
}

func (m *Model) stopContainerStream() {
	if m.containerStream != nil {
		m.containerStream.Stop()
		m.containerStream = nil
	}
}

// restartSelected mata y vuelve a arrancar el proceso seleccionado (como el
// "r" de mprocs). No hace nada si lo seleccionado es un contenedor de
// infra — esos se manejan con "hels env", no desde acá.
func (m *Model) restartSelected() tea.Cmd {
	if len(m.items) == 0 || m.items[m.cursor].kind != kindProcess {
		return nil
	}
	name := strings.TrimPrefix(m.items[m.cursor].key, "proc:")
	pe := m.findProcess(name)
	if pe == nil {
		return nil
	}

	if pe.handle != nil {
		pe.handle.Stop()
	}
	pe.lines = nil
	pe.exitErr = nil
	pe.gen++
	gen := pe.gen

	handle, err := startProcess(pe.spec.Cmd, pe.spec.Dir)
	if err != nil {
		pe.running = false
		pe.exitErr = err
		pe.handle = nil
		m.rebuildItems()
		m.refreshLogViewport()
		return nil
	}
	pe.handle = handle
	pe.running = true
	m.rebuildItems()
	m.refreshLogViewport()

	return waitForProcLines(name, handle, gen)
}

// stopEverything para todos los procesos locales al salir del dashboard,
// para no dejar huérfano un gateway o un frontend corriendo en segundo
// plano después de cerrar hels.
func (m *Model) stopEverything() {
	m.stopContainerStream()
	for _, pe := range m.processes {
		if pe.handle != nil {
			pe.handle.Stop()
		}
	}
}

// waitForLogLines devuelve un tea.Cmd que espera al menos una línea nueva
// del stream de un CONTENEDOR y de paso agrupa (drena, sin bloquear)
// cualquier otra línea que ya esté esperando en el canal. Esto importa
// mucho para un contenedor con historial largo (ej. sshd con cientos de
// conexiones logueadas): al seleccionarlo, "docker logs --tail N" entrega
// ese historial casi de golpe, y sin agrupar cada línea dispara su propio
// ciclo completo de Update+render — una ráfaga de cientos de mensajes en
// milisegundos que satura el render y rompe la vista. Agrupando, esa misma
// ráfaga se procesa en uno o pocos renders.
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

// waitForProcLines es el equivalente de waitForLogLines para un proceso
// local: agrupa ráfagas de líneas de la misma forma, pero identificadas por
// nombre+generación para poder procesarse SIEMPRE (no solo cuando ese
// proceso está seleccionado), porque sigue corriendo en segundo plano.
func waitForProcLines(name string, h *procHandle, gen int) tea.Cmd {
	if h == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-h.Lines
		if !ok {
			err := <-h.Done
			return procExitMsg{name: name, gen: gen, err: err}
		}

		lines := []string{line}
	drain:
		for {
			select {
			case l, ok := <-h.Lines:
				if !ok {
					break drain
				}
				lines = append(lines, l)
			default:
				break drain
			}
		}

		return procLinesMsg{name: name, gen: gen, lines: lines}
	}
}

// currentLogLines devuelve el buffer del ítem actualmente seleccionado: el
// de un proceso (persistente, sigue corriendo en segundo plano) o el del
// stream de un contenedor (on-demand).
func (m *Model) currentLogLines() []string {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if item.kind == kindProcess {
		if pe := m.findProcess(strings.TrimPrefix(item.key, "proc:")); pe != nil {
			return pe.lines
		}
		return nil
	}
	return m.containerLogLines
}

// currentLogErr devuelve el error a mostrar en la fila de estado del panel
// de logs para el ítem seleccionado (el proceso terminó con error, o el
// stream del contenedor cortó con error).
func (m *Model) currentLogErr() error {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if item.kind == kindProcess {
		if pe := m.findProcess(strings.TrimPrefix(item.key, "proc:")); pe != nil {
			return pe.exitErr
		}
		return nil
	}
	return m.containerLogErr
}

// refreshLogViewport reconstruye el contenido visible del panel de logs a
// partir de currentLogLines().
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

	lines := m.currentLogLines()
	var display []string
	for i, l := range lines {
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
