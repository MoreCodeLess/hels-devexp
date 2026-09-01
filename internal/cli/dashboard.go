package cli

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	hlenv "github.com/MoreCodeLess/hels-devexp/internal/env"
	"github.com/MoreCodeLess/hels-devexp/internal/tui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Abre la ventana interactiva: procesos locales, infra y logs en vivo",
	Long: `Abre un dashboard en la terminal, al estilo mprocs: a la izquierda la
lista de servicios (arriba los procesos locales declarados en
processes.* de hels.yaml — gateway, frontend, lo que sea; abajo la
infraestructura, hoy contenedores Docker), y a la derecha los logs en
vivo del seleccionado.

Los procesos de hels.yaml arrancan solos al abrir el dashboard y siguen
corriendo en segundo plano aunque estés mirando otra cosa — igual que en
mprocs. Si no hay hels.yaml en el directorio actual (o no declara
"processes"), el dashboard igual funciona mostrando solo la infra Docker.

Click en un servicio de la lista lo selecciona. Click en el panel de logs
(o Tab) le pasa el foco del teclado, para poder scrollear con las flechas,
j/k, pgup/pgdn, etc. La rueda del mouse siempre scrollea los logs según
dónde esté el cursor, sin depender del foco. "r" reinicia el proceso
seleccionado. Salí con q (para todos los procesos locales antes de cerrar).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specs := loadProcessSpecs()
		endpoint := loadLambdaEndpoint()

		m := tui.New(specs, endpoint)
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		m.AttachProgram(p)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("corriendo el dashboard: %w", err)
		}
		return nil
	},
}

// loadProcessSpecs busca un hels.yaml en el directorio actual y devuelve sus
// processes.* como specs para el dashboard. Si no hay hels.yaml, o no
// declara processes, devuelve nil sin error — el dashboard funciona igual,
// solo con la infra Docker.
func loadProcessSpecs() []tui.ProcessSpec {
	cfg, err := loadProjectConfig()
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(cfg.Processes))
	for name := range cfg.Processes {
		names = append(names, name)
	}
	sort.Strings(names)

	specs := make([]tui.ProcessSpec, 0, len(names))
	for _, name := range names {
		p := cfg.Processes[name]
		specs = append(specs, tui.ProcessSpec{Name: name, Cmd: p.Cmd, Dir: p.Dir})
	}
	return specs
}

// loadLambdaEndpoint busca, entre los entornos declarados en hels.yaml, uno
// que esté corriendo ahora mismo, y devuelve su URL para que el dashboard
// pueda listar sus funciones Lambda. Prioriza el entorno "activo" (el que
// dejó `hels env switch`); si no hay ninguno activo pero igual hay uno
// corriendo, usa ese. Devuelve "" si no hay hels.yaml, no declara ningún
// entorno, o ninguno está arriba — el dashboard funciona igual, solo sin la
// sección de Lambdas.
func loadLambdaEndpoint() string {
	cfg, err := loadProjectConfig()
	if err != nil {
		return ""
	}

	statuses, err := hlenv.List(cfg)
	if err != nil {
		return ""
	}

	state, _ := hlenv.LoadState()

	var firstRunning *hlenv.Status
	for _, st := range statuses {
		if !st.Running {
			continue
		}
		if state != nil && st.Name == state.Active {
			return st.EndpointURL()
		}
		if firstRunning == nil {
			firstRunning = st
		}
	}
	if firstRunning != nil {
		return firstRunning.EndpointURL()
	}
	return ""
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
