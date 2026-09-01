package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MoreCodeLess/hels-devexp/internal/scanner"
)

var scanFormat string

var scanCmd = &cobra.Command{
	Use:   "scan <ruta>",
	Short: "Busca servicios de Serverless Framework bajo una ruta y mapea cómo se conectan",
	Long: `Recorre <ruta> buscando archivos serverless.yml/serverless.yaml, extrae
las funciones y recursos de cada servicio, y detecta conexiones entre
servicios vía referencias cruzadas de CloudFormation (${cf:...}),
Fn::ImportValue, y ARNs/nombres de recursos compartidos (colas, tópicos,
tablas, buckets).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := args[0]

		paths, err := scanner.Discover(root)
		if err != nil {
			return fmt.Errorf("buscando serverless.yml bajo %s: %w", root, err)
		}
		if len(paths) == 0 {
			return fmt.Errorf("no se encontró ningún serverless.yml/serverless.yaml bajo %s", root)
		}

		var services []*scanner.ServiceDef
		for _, path := range paths {
			svc, err := scanner.ParseFile(path)
			if err != nil {
				return err
			}
			services = append(services, svc)
		}

		graph := scanner.BuildGraph(services)

		switch scanFormat {
		case "text":
			scanner.RenderText(cmd.OutOrStdout(), graph)
		case "json":
			return scanner.RenderJSON(cmd.OutOrStdout(), graph)
		case "mermaid":
			scanner.RenderMermaid(cmd.OutOrStdout(), graph)
		default:
			return fmt.Errorf("--format %q inválido (usá text, json o mermaid)", scanFormat)
		}
		return nil
	},
}

func init() {
	scanCmd.Flags().StringVar(&scanFormat, "format", "text", "Formato de salida: text, json o mermaid")
	rootCmd.AddCommand(scanCmd)
}
