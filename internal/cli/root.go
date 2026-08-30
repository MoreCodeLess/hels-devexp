// Package cli define los comandos de la CLI de hels.
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "hels",
	Short: "Simulá tu nube AWS localmente para equipos de Serverless Framework",
	Long: `hels orquesta entornos locales reproducibles sobre floci (un emulador
de AWS) a partir de un único archivo hels.yaml: levantar y cambiar entornos,
correr pruebas y, en fases futuras, mapear servicios de Serverless Framework,
unificar logs y sumar herramientas locales externas a una misma vista.`,
}

// Execute corre el comando raíz de la CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(upgradeCmd)
}
