package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/MoreCodeLess/hels-devexp/internal/tui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Abre la ventana interactiva: servicios locales y sus logs en vivo",
	Long: `Abre un dashboard en la terminal con la lista de servicios locales
(hoy, contenedores Docker corriendo) a la izquierda y los logs en vivo del
seleccionado a la derecha, al estilo mprocs.

Click en un servicio de la lista lo selecciona. Click en el panel de logs
(o Tab) le pasa el foco del teclado, para poder scrollear con las flechas,
j/k, pgup/pgdn, etc. La rueda del mouse siempre scrollea los logs según
dónde esté el cursor, sin depender del foco. Salí con q.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.New()
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		m.AttachProgram(p)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("corriendo el dashboard: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
