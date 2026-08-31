package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
	hlenv "github.com/MoreCodeLess/hels-devexp/internal/env"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Maneja el ciclo de vida de los entornos locales declarados en hels.yaml",
}

var envUpCmd = &cobra.Command{
	Use:   "up <entorno>",
	Short: "Levanta un entorno (o confirma que ya está corriendo)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}
		st, err := hlenv.Up(cfg, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s está corriendo en %s\n", st.Name, st.EndpointURL())
		return nil
	},
}

var envDownCmd = &cobra.Command{
	Use:   "down <entorno>",
	Short: "Baja un entorno",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}
		if err := hlenv.Down(cfg, args[0]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s está abajo\n", args[0])
		return nil
	},
}

var envStatusCmd = &cobra.Command{
	Use:   "status <entorno>",
	Short: "Muestra el estado de un entorno",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}
		st, err := hlenv.StatusOf(cfg, args[0])
		if err != nil {
			return err
		}
		printStatus(cmd, st)
		return nil
	},
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todos los entornos declarados y su estado",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}
		statuses, err := hlenv.List(cfg)
		if err != nil {
			return err
		}

		state, err := hlenv.LoadState()
		if err != nil {
			return err
		}

		for _, st := range statuses {
			marker := "  "
			if st.Name == state.Active {
				marker = "* "
			}
			up := "abajo"
			if st.Running {
				up = "arriba (" + st.EndpointURL() + ")"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s%s\t%s\n", marker, st.Name, up)
		}
		return nil
	},
}

var envSwitchCmd = &cobra.Command{
	Use:   "switch <entorno>",
	Short: "Levanta un entorno (si hace falta) y lo marca como el activo",
	Long: `Levanta el entorno pedido si no está corriendo, lo marca como el
entorno activo en esta máquina, e imprime a stdout las variables de
entorno para apuntar el AWS SDK/CLI ahí. Pensado para usar con eval:

  eval "$(hels env switch dev)"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envName := args[0]

		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Levantando %s...\n", envName)
		st, err := hlenv.Up(cfg, envName)
		if err != nil {
			return err
		}

		state, err := hlenv.LoadState()
		if err != nil {
			return err
		}
		state.Active = envName
		if err := hlenv.SaveState(state); err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "export AWS_ENDPOINT_URL=%s\n", st.EndpointURL())
		fmt.Fprintf(out, "export AWS_DEFAULT_REGION=%s\n", st.Region)
		fmt.Fprintln(out, "export AWS_ACCESS_KEY_ID=test")
		fmt.Fprintln(out, "export AWS_SECRET_ACCESS_KEY=test")

		fmt.Fprintf(cmd.ErrOrStderr(), "%s activo. Corré: eval \"$(hels env switch %s)\"\n", envName, envName)
		return nil
	},
}

func printStatus(cmd *cobra.Command, st *hlenv.Status) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "entorno:    %s\n", st.Name)
	fmt.Fprintf(out, "contenedor: %s\n", st.Container)
	if st.Running {
		fmt.Fprintf(out, "estado:     arriba\n")
		fmt.Fprintf(out, "endpoint:   %s\n", st.EndpointURL())
	} else {
		fmt.Fprintf(out, "estado:     abajo\n")
	}
	fmt.Fprintf(out, "región:     %s\n", st.Region)
	fmt.Fprintf(out, "cuenta:     %s\n", st.AccountID)
	fmt.Fprintf(out, "storage:    %s\n", st.Storage)
}

// loadProjectConfig busca y carga hels.yaml en el directorio actual.
func loadProjectConfig() (*config.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(wd, config.DefaultFileName)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no se encontró %s en %s — corré 'hels init' primero", config.DefaultFileName, wd)
	}

	return config.Load(path)
}

func init() {
	envCmd.AddCommand(envUpCmd, envDownCmd, envStatusCmd, envListCmd, envSwitchCmd)
	rootCmd.AddCommand(envCmd)
}
