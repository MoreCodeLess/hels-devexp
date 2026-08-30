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
(hoy, contenedores Docker corriendo) y los logs en vivo del que tengas
seleccionado. Navegá con las flechas o j/k, salí con q.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.New()
		p := tea.NewProgram(m, tea.WithAltScreen())
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
