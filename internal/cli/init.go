package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Crea un hels.yaml de partida en el directorio actual",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		path := filepath.Join(wd, config.DefaultFileName)

		if _, err := os.Stat(path); err == nil && !initForce {
			return fmt.Errorf("%s ya existe (usá --force para sobreescribir)", config.DefaultFileName)
		}

		if err := os.WriteFile(path, []byte(config.ExampleYAML), 0o644); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Creado %s\n", path)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Sobreescribe hels.yaml si ya existe")
}
