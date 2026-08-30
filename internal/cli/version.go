package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version se sobreescribe en build time con -ldflags "-X .../internal/cli.Version=x.y.z".
var Version = "0.0.0-dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Muestra la versión de hels",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	},
}
