package tui

import (
	"fmt"
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
	categoryTabsRow    = 1 + paneTitleLines     // 1 = borde-top del panel
	listItemsTopRow    = categoryTabsRow + 1    // +1 = la fila de categorías
	listContentLeftCol = 2                      // borde izq (1) + padding izq (1) del panel
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
	kindLambda
	kindQueue
	kindTopic
)

// kindAll es el valor de categoryFilter que significa "sin filtrar, mostrar
// todo" — no es un itemKind real (ningún listItem lo usa), por eso el -1.
const kindAll itemKind = -1

// itemIcon es el ícono de una categoría, compartido entre el menú de
// categorías y cada fila de la lista para que sean siempre coherentes.
//
// A propósito son todos caracteres ASCII (salvo λ, que es una letra griega
// normal — ancho angosto garantizado en cualquier terminal/fuente). Antes
// se usaban símbolos del bloque de formas geométricas (▶ ● ▤ ◎), que son
// "ancho ambiguo" en Unicode (según el terminal/fuente rinden a 1 o 2
// celdas) — no resultó ser la causa del desborde de una fila que se veía
// con la lista (esa era otra cosa, ver el comentario en renderList sobre
// Width() redundante), pero de todas formas es más robusto no depender de
// que la terminal del usuario los interprete como angostos.
func itemIcon(k itemKind) string {
	switch k {
	case kindContainer:
		return "#"
	case kindLambda:
		return "λ"
	case kindQueue:
		return "Q"
	case kindTopic:
		return "T"
	case kindAll:
		return "*"
	default:
		return ">"
	}
}

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

// lambdasLoadedMsg trae la lista de funciones Lambda desplegadas contra el
// entorno floci activo (si hay uno declarado en hels.yaml y corriendo).
type lambdasLoadedMsg struct {
	functions []LambdaFunction
	err       error
}

// messagingLoadedMsg trae las colas SQS y tópicos SNS del entorno floci
// activo. Es una consulta estructural (ListQueues/GetQueueAttributes,
// ListTopics/ListSubscriptionsByTopic) — no consume ningún mensaje, así que
// a diferencia del peek de una cola seleccionada, esta se puede repetir en
// cada refresh sin ningún efecto secundario.
type messagingLoadedMsg struct {
	queues []QueueInfo
	topics []TopicInfo
	err    error
}

// queuePeekMsg trae el resultado de "espiar" (ver peekQueueMessages) la cola
// seleccionada ahora mismo.
type queuePeekMsg struct {
	gen   int
	lines []string
	err   error
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

	// categoryFilter restringe qué categoría de la lista se muestra
	// (kindAll = todas). listScroll es el índice del primer ítem visible
	// DENTRO de esa vista filtrada — la lista es una ventana de tamaño fijo,
	// igual que el panel de logs, así que con muchos ítems hay que
	// scrollear en vez de dejar que el panel crezca sin límite.
	categoryFilter itemKind
	listScroll     int

	// flociEndpoint es la URL de floci contra la que preguntamos qué
	// funciones Lambda/colas SQS/tópicos SNS hay desplegados ("" si no hay
	// hels.yaml o ningún entorno declarado está corriendo — en ese caso no
	// se muestran esas secciones).
	flociEndpoint string
	lambdas       []LambdaFunction
	queues        []QueueInfo
	topics        []TopicInfo

	// Estado del stream de logs de un CONTENEDOR o LAMBDA seleccionado
	// (on-demand: se arranca al seleccionar, se para al deseleccionar). Los
	// procesos no usan esto — su buffer vive en procEntry.lines. Para una
	// Lambda, streamContainerID es el contenedor Docker efímero que floci
	// tiene corriendo AHORA para esa función (puede cambiar de identidad
	// mientras está seleccionada, si el warm pool de floci lo recicla — ver
	// refreshLambdaStream).
	containerLogLines []string
	containerLogGen   int
	containerStream   *logStream
	containerLogErr   error
	streamContainerID string

	viewport viewport.Model
	ready    bool
	err      error

	width, height int
}

// New crea el modelo inicial del dashboard. specs son los procesos locales
// declarados en hels.yaml (processes.*) — puede ser nil/vacío si no hay
// hels.yaml o no declara ninguno; el dashboard sigue funcionando mostrando
// solo la infraestructura Docker. flociEndpoint es la URL del floci activo
// (ej. "http://localhost:4566") para listar sus funciones Lambda, colas SQS
// y tópicos SNS — "" si no hay ninguno corriendo, en cuyo caso esas
// secciones del dashboard no aparecen.
func New(specs []ProcessSpec, flociEndpoint string) *Model {
	return &Model{processSpecs: specs, flociEndpoint: flociEndpoint, categoryFilter: kindAll}
}

// AttachProgram le da al modelo una referencia al *tea.Program que lo corre,
// para que los goroutines de streaming de logs puedan mandarle mensajes.
// Hay que llamarlo antes de p.Run().
func (m *Model) AttachProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadContainersCmd(), tickCmd()}
	if m.flociEndpoint != "" {
		cmds = append(cmds, loadLambdasCmd(m.flociEndpoint), loadMessagingCmd(m.flociEndpoint))
	}
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

	if len(m.lambdas) > 0 {
		// Una sola consulta a Docker para todas las Lambdas, en vez de una
		// por función (ver matchLambdaContainer).
		lambdaContainers, _ := listLambdaContainers()
		for _, fn := range m.lambdas {
			status := "sin invocaciones recientes"
			ok := false
			if c, found := matchLambdaContainer(lambdaContainers, fn.Name); found {
				status = c.Status
				ok = true
			}
			items = append(items, listItem{
				kind: kindLambda, key: "lambda:" + fn.Name, name: fn.Name, status: status, ok: ok,
			})
		}
	}

	for _, q := range m.queues {
		status := fmt.Sprintf("%d visibles, %d en vuelo", q.Visible, q.InFlight)
		items = append(items, listItem{
			kind: kindQueue, key: "queue:" + q.Name, name: q.Name, status: status, ok: true,
		})
	}

	for _, t := range m.topics {
		status := fmt.Sprintf("%d suscriptor(es)", len(t.Subscriptions))
		items = append(items, listItem{
			kind: kindTopic, key: "topic:" + t.Name, name: t.Name, status: status, ok: true,
		})
	}

	m.items = items
	m.clampCursor()
}

// visibleItems devuelve m.items filtrados por categoryFilter (o todos, si es
// kindAll). m.cursor y m.listScroll son siempre índices dentro de ESTA
// vista, no de m.items — así que cambiar de categoría cambia también qué
// significan.
func (m *Model) visibleItems() []listItem {
	if m.categoryFilter == kindAll {
		return m.items
	}
	out := make([]listItem, 0, len(m.items))
	for _, it := range m.items {
		if it.kind == m.categoryFilter {
			out = append(out, it)
		}
	}
	return out
}

// availableCategories devuelve, en un orden fijo, las categorías que tienen
// al menos un ítem ahora mismo — así el menú no muestra pestañas vacías
// (ej. "Colas" antes de que exista ninguna).
func (m *Model) availableCategories() []itemKind {
	seen := make(map[itemKind]bool)
	for _, it := range m.items {
		seen[it.kind] = true
	}
	order := []itemKind{kindProcess, kindContainer, kindLambda, kindQueue, kindTopic}
	out := make([]itemKind, 0, len(order))
	for _, k := range order {
		if seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// categoryTab es una pestaña clickeable del menú de categorías, con su
// posición horizontal exacta dentro del panel — la usan tanto el render
// (view.go) como el hit-test del click, para que nunca se desalineen.
type categoryTab struct {
	kind     itemKind
	label    string
	startCol int // inclusive, columna dentro del contenido del panel
	endCol   int // exclusivo
}

func (m *Model) categoryTabsLayout() []categoryTab {
	kinds := append([]itemKind{kindAll}, m.availableCategories()...)
	tabs := make([]categoryTab, 0, len(kinds))
	col := 0
	for _, k := range kinds {
		label := "[" + itemIcon(k) + "]"
		width := len([]rune(label))
		tabs = append(tabs, categoryTab{kind: k, label: label, startCol: col, endCol: col + width})
		col += width + 1 // 1 espacio de separación entre pestañas
	}
	return tabs
}

// maxVisibleListItems es cuántos ítems (de a 2 filas cada uno) entran en la
// ventana fija de la lista, dado el alto actual de la terminal.
func (m *Model) maxVisibleListItems() int {
	n := m.viewport.Height / 2
	if n < 1 {
		n = 1
	}
	return n
}

// clampCursor mantiene m.cursor dentro de los límites de la vista filtrada
// actual (por ejemplo, después de un refresh que la achicó, o de cambiar de
// categoría) y de paso re-ajusta el scroll para que el cursor siga visible.
func (m *Model) clampCursor() {
	n := len(m.visibleItems())
	if m.cursor >= n {
		m.cursor = maxInt(0, n-1)
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

// ensureCursorVisible corre m.listScroll lo mínimo necesario para que
// m.cursor quede dentro de la ventana visible — el mismo tipo de
// "scroll-into-view" de cualquier lista con selección.
func (m *Model) ensureCursorVisible() {
	max := m.maxVisibleListItems()
	if m.cursor < m.listScroll {
		m.listScroll = m.cursor
	} else if m.cursor >= m.listScroll+max {
		m.listScroll = m.cursor - max + 1
	}
	m.clampListScroll()
}

// scrollListBy mueve la ventana de la lista sin tocar la selección (para la
// rueda del mouse, que deja mirar otros ítems sin perder cuál está elegido).
func (m *Model) scrollListBy(delta int) {
	m.listScroll += delta
	m.clampListScroll()
}

func (m *Model) clampListScroll() {
	if m.listScroll < 0 {
		m.listScroll = 0
	}
	maxOffset := len(m.visibleItems()) - m.maxVisibleListItems()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.listScroll > maxOffset {
		m.listScroll = maxOffset
	}
}

// setCategoryFilter cambia qué categoría se muestra y arranca de nuevo la
// selección desde el principio de la vista nueva (los índices viejos no
// tienen sentido en la categoría nueva).
func (m *Model) setCategoryFilter(k itemKind) tea.Cmd {
	if m.categoryFilter == k {
		return nil
	}
	m.categoryFilter = k
	m.cursor = 0
	m.listScroll = 0
	return m.ensureSelection()
}

// cycleCategoryFilter pasa a la categoría siguiente/anterior (delta ±1),
// para poder cambiar de categoría también con el teclado.
func (m *Model) cycleCategoryFilter(delta int) tea.Cmd {
	kinds := append([]itemKind{kindAll}, m.availableCategories()...)
	curIdx := 0
	for i, k := range kinds {
		if k == m.categoryFilter {
			curIdx = i
			break
		}
	}
	next := ((curIdx+delta)%len(kinds) + len(kinds)) % len(kinds)
	return m.setCategoryFilter(kinds[next])
}

func (m *Model) findQueue(name string) *QueueInfo {
	for i := range m.queues {
		if m.queues[i].Name == name {
			return &m.queues[i]
		}
	}
	return nil
}

func (m *Model) findTopic(name string) *TopicInfo {
	for i := range m.topics {
		if m.topics[i].Name == name {
			return &m.topics[i]
		}
	}
	return nil
}

// formatTopicDetail arma las líneas del panel de logs para un tópico SNS
// seleccionado. SNS no retiene histórico de publicaciones — lo único
// inspeccionable es a quién reenvía lo que se publique ahí.
func formatTopicDetail(t *TopicInfo) []string {
	if t == nil {
		return []string{"(tópico no encontrado)"}
	}
	if len(t.Subscriptions) == 0 {
		return []string{"(sin suscriptores)"}
	}
	lines := make([]string, 0, len(t.Subscriptions)+1)
	lines = append(lines, fmt.Sprintf("%d suscriptor(es):", len(t.Subscriptions)))
	for _, s := range t.Subscriptions {
		lines = append(lines, "  - "+s)
	}
	return lines
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

func loadLambdasCmd(endpoint string) tea.Cmd {
	return func() tea.Msg {
		fns, err := listLambdaFunctions(endpoint)
		return lambdasLoadedMsg{functions: fns, err: err}
	}
}

func loadMessagingCmd(endpoint string) tea.Cmd {
	return func() tea.Msg {
		qs, qerr := listQueues(endpoint)
		ts, terr := listTopics(endpoint)
		err := qerr
		if err == nil {
			err = terr
		}
		return messagingLoadedMsg{queues: qs, topics: ts, err: err}
	}
}

// peekQueueCmd espía (ver peekQueueMessages) la cola q y devuelve las líneas
// ya formateadas para el panel de logs.
func peekQueueCmd(endpoint string, q QueueInfo, gen int) tea.Cmd {
	return func() tea.Msg {
		msgs, err := peekQueueMessages(endpoint, q.URL, 10)
		if err != nil {
			return queuePeekMsg{gen: gen, err: err}
		}
		return queuePeekMsg{gen: gen, lines: formatPeekedMessages(msgs)}
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

	case lambdasLoadedMsg:
		if msg.err == nil {
			m.lambdas = msg.functions
		}
		m.rebuildItems()
		return m, m.ensureSelection()

	case messagingLoadedMsg:
		if msg.err == nil {
			m.queues = msg.queues
			m.topics = msg.topics
		}
		m.rebuildItems()
		// Un tópico seleccionado no dispara un pedido propio (su detalle sale
		// de esta misma carga) — si es lo que se está mirando, refrescamos el
		// panel de logs de una con el dato nuevo.
		if visible := m.visibleItems(); len(visible) > 0 && m.cursor < len(visible) && visible[m.cursor].kind == kindTopic {
			name := strings.TrimPrefix(visible[m.cursor].key, "topic:")
			m.containerLogLines = formatTopicDetail(m.findTopic(name))
			m.refreshLogViewport()
		}
		return m, m.ensureSelection()

	case queuePeekMsg:
		if msg.gen != m.containerLogGen {
			return m, nil
		}
		if msg.err != nil {
			m.containerLogErr = msg.err
		} else {
			m.containerLogErr = nil
			m.containerLogLines = msg.lines
		}
		m.refreshLogViewport()
		return m, nil

	case refreshTickMsg:
		cmds := []tea.Cmd{loadContainersCmd(), tickCmd()}
		if m.flociEndpoint != "" {
			cmds = append(cmds, loadLambdasCmd(m.flociEndpoint), loadMessagingCmd(m.flociEndpoint))
		}
		// Si lo seleccionado es una Lambda, el contenedor efímero que muestra
		// pudo haber cambiado de identidad (floci lo recicló por inactividad
		// y lo volvió a crear en otra invocación) o haber aparecido/
		// desaparecido — hay que re-engancharse al que esté vivo ahora.
		if cmd := m.refreshSelectedLambdaStream(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Si lo seleccionado es una cola, volvemos a espiarla para que el
		// panel de logs se sienta "en vivo" (llegan mensajes nuevos, otros
		// consumidores se llevan los que había).
		if cmd := m.refreshSelectedQueuePeek(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

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
					m.ensureCursorVisible()
					return m, m.ensureSelection()
				}
			case "down", "j":
				if m.cursor < len(m.visibleItems())-1 {
					m.cursor++
					m.ensureCursorVisible()
					return m, m.ensureSelection()
				}
			case "left", "h":
				return m, m.cycleCategoryFilter(-1)
			case "right", "l":
				return m, m.cycleCategoryFilter(1)
			}
			return m, nil
		}
		// focusLogs: cualquier tecla (arriba/abajo/j/k, pgup/pgdn, home/end,
		// ctrl+u/d, ...) se la pasamos directo al viewport para que scrollee.

	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			if k, ok := m.hitTestCategoryTabs(msg.X, msg.Y); ok {
				m.focus = focusList
				return m, m.setCategoryFilter(k)
			}
			if idx, ok := m.hitTestList(msg.X, msg.Y); ok {
				m.focus = focusList
				m.cursor = idx
				return m, m.ensureSelection()
			}
			m.focus = focusLogs
			return m, nil

		case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
			// sobre la lista, la rueda scrollea la ventana de servicios sin
			// tocar la selección; en cualquier otro lado, scrollea los logs
			// sin depender de qué panel tenga el foco del teclado.
			if msg.X < listPaneOuterWidth {
				delta := 2
				if msg.Button == tea.MouseButtonWheelUp {
					delta = -2
				}
				m.scrollListBy(delta)
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

// hitTestList traduce coordenadas de mouse a un índice DENTRO DE
// visibleItems() (no de m.items — ver esa función). Devuelve ok=false si el
// click cayó fuera del panel de lista, sobre una fila sin servicio, o más
// abajo de lo que la ventana de scroll tiene renderizado ahora mismo. Cada
// ítem ocupa 2 filas (nombre + estado, ver renderList en view.go), por eso
// se divide entre 2.
func (m *Model) hitTestList(x, y int) (int, bool) {
	if x < 0 || x >= listPaneOuterWidth {
		return 0, false
	}
	rowOffset := y - listItemsTopRow
	if rowOffset < 0 {
		return 0, false
	}
	row := rowOffset / 2
	if row >= m.maxVisibleListItems() {
		return 0, false
	}
	idx := m.listScroll + row
	if idx >= len(m.visibleItems()) {
		return 0, false
	}
	return idx, true
}

// hitTestCategoryTabs traduce coordenadas de mouse a qué pestaña del menú de
// categorías (si alguna) cayó bajo el click — comparte el layout exacto con
// el render (categoryTabsLayout) para que nunca se desalineen.
func (m *Model) hitTestCategoryTabs(x, y int) (itemKind, bool) {
	if y != categoryTabsRow || x < 0 || x >= listPaneOuterWidth {
		return kindAll, false
	}
	col := x - listContentLeftCol
	if col < 0 {
		return kindAll, false
	}
	for _, t := range m.categoryTabsLayout() {
		if col >= t.startCol && col < t.endCol {
			return t.kind, true
		}
	}
	return kindAll, false
}

// ensureSelection arranca (si hace falta) el stream de logs del ítem bajo
// el cursor. Para un proceso no hay nada que "arrancar": ya viene corriendo
// solo desde Init, así que solo cambia qué buffer se muestra.
func (m *Model) ensureSelection() tea.Cmd {
	visible := m.visibleItems()
	if len(visible) == 0 {
		m.selectedKey = ""
		m.stopContainerStream()
		return nil
	}
	if m.cursor >= len(visible) {
		m.cursor = len(visible) - 1
	}
	target := visible[m.cursor]
	if target.key == m.selectedKey {
		return nil
	}

	m.stopContainerStream()
	m.selectedKey = target.key

	if target.kind == kindProcess {
		m.refreshLogViewport()
		return nil
	}

	if target.kind == kindLambda {
		name := strings.TrimPrefix(target.key, "lambda:")
		containers, _ := listLambdaContainers()
		c, found := matchLambdaContainer(containers, name)
		if !found {
			m.streamContainerID = ""
			m.containerLogLines = []string{lambdaColdMessage}
			m.containerLogErr = nil
			m.refreshLogViewport()
			return nil
		}
		return m.startContainerStream(c.ID)
	}

	if target.kind == kindQueue {
		name := strings.TrimPrefix(target.key, "queue:")
		q := m.findQueue(name)
		if q == nil {
			m.containerLogLines = []string{"(cola no encontrada)"}
			m.containerLogErr = nil
			m.refreshLogViewport()
			return nil
		}
		m.containerLogLines = nil
		m.containerLogErr = nil
		m.containerLogGen++
		return peekQueueCmd(m.flociEndpoint, *q, m.containerLogGen)
	}

	if target.kind == kindTopic {
		name := strings.TrimPrefix(target.key, "topic:")
		m.containerLogLines = formatTopicDetail(m.findTopic(name))
		m.containerLogErr = nil
		m.refreshLogViewport()
		return nil
	}

	// kindContainer: la key ya es el ID del contenedor.
	return m.startContainerStream(target.key)
}

// startContainerStream arranca "docker logs -f" para el contenedor id y lo
// deja como el stream on-demand activo (usado tanto para infra como para el
// contenedor efímero de una Lambda). streamContainerID queda registrado para
// que refreshSelectedLambdaStream pueda notar si cambia de identidad.
func (m *Model) startContainerStream(id string) tea.Cmd {
	m.containerLogLines = nil
	m.containerLogErr = nil
	m.containerLogGen++
	gen := m.containerLogGen
	m.streamContainerID = id

	stream, err := startLogStream(id, tailLines)
	if err != nil {
		m.containerLogErr = err
		m.refreshLogViewport()
		return nil
	}
	m.containerStream = stream
	m.refreshLogViewport()

	return waitForLogLines(stream, gen)
}

// refreshSelectedLambdaStream se llama en cada refreshTick. Si lo
// seleccionado es una Lambda, floci puede haber reciclado su contenedor
// efímero (warm pool) desde la última vez: mismo nombre de función, otro
// container ID, o directamente ninguno. Si detecta un cambio, reengancha el
// stream al contenedor vivo actual (o muestra el mensaje de "función fría"
// si ya no hay ninguno).
func (m *Model) refreshSelectedLambdaStream() tea.Cmd {
	visible := m.visibleItems()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return nil
	}
	item := visible[m.cursor]
	if item.kind != kindLambda {
		return nil
	}

	name := strings.TrimPrefix(item.key, "lambda:")
	containers, _ := listLambdaContainers()
	c, found := matchLambdaContainer(containers, name)

	switch {
	case found && c.ID == m.streamContainerID:
		return nil // sigue siendo el mismo contenedor, no hay nada que hacer
	case found:
		m.stopContainerStream()
		return m.startContainerStream(c.ID)
	case m.streamContainerID != "":
		// Tenía un contenedor y ya no está (floci lo bajó por inactividad).
		m.stopContainerStream()
		m.streamContainerID = ""
		m.containerLogLines = []string{lambdaColdMessage}
		m.refreshLogViewport()
	}
	return nil
}

// refreshSelectedQueuePeek se llama en cada refreshTick. Si lo seleccionado
// es una cola, la vuelve a espiar para que el panel de logs muestre lo que
// hay AHORA (mensajes nuevos que llegaron, otros que ya se consumieron en
// otro lado) en vez de quedarse pegado al primer peek.
func (m *Model) refreshSelectedQueuePeek() tea.Cmd {
	visible := m.visibleItems()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return nil
	}
	item := visible[m.cursor]
	if item.kind != kindQueue {
		return nil
	}
	q := m.findQueue(strings.TrimPrefix(item.key, "queue:"))
	if q == nil {
		return nil
	}
	m.containerLogGen++
	return peekQueueCmd(m.flociEndpoint, *q, m.containerLogGen)
}

func (m *Model) stopContainerStream() {
	if m.containerStream != nil {
		m.containerStream.Stop()
		m.containerStream = nil
	}
	m.streamContainerID = ""
}

// lambdaColdMessage se muestra en el panel de logs cuando se selecciona una
// Lambda que no tiene ningún contenedor "caliente" corriendo ahora mismo
// (floci lo bajó por inactividad, o todavía no se invocó nunca).
const lambdaColdMessage = "(sin invocaciones activas ahora mismo — invocá la función para ver sus logs acá; el dashboard se reengancha solo)"

// restartSelected mata y vuelve a arrancar el proceso seleccionado (como el
// "r" de mprocs). No hace nada si lo seleccionado es un contenedor de
// infra — esos se manejan con "hels env", no desde acá.
func (m *Model) restartSelected() tea.Cmd {
	visible := m.visibleItems()
	if len(visible) == 0 || m.cursor >= len(visible) || visible[m.cursor].kind != kindProcess {
		return nil
	}
	name := strings.TrimPrefix(visible[m.cursor].key, "proc:")
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
	visible := m.visibleItems()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return nil
	}
	item := visible[m.cursor]
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
	visible := m.visibleItems()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return nil
	}
	item := visible[m.cursor]
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
