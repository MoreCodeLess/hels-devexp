package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
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
tablas, buckets).

Con --format hels-yaml genera un hels.yaml de partida a partir de lo
encontrado (un solo entorno "dev", con los servicios de floci que hacen
falta para simular todo lo detectado) — revisalo antes de usarlo, es un
punto de partida, no una verdad absoluta:

  hels scan ./mi-monorepo --format hels-yaml > hels.yaml`,
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
		case "hels-yaml":
			return renderHelsYAML(cmd.OutOrStdout(), root, graph)
		default:
			return fmt.Errorf("--format %q inválido (usá text, json, mermaid o hels-yaml)", scanFormat)
		}
		return nil
	},
}

func init() {
	scanCmd.Flags().StringVar(&scanFormat, "format", "text", "Formato de salida: text, json, mermaid o hels-yaml")
	rootCmd.AddCommand(scanCmd)
}

// renderHelsYAML arma un hels.yaml de partida a partir de lo que encontró el
// escáner: un solo entorno "dev" con los servicios de floci que hacen falta
// para simular localmente todo lo que se detectó (ver scanner.DetectAWSServices).
func renderHelsYAML(w io.Writer, root string, g *scanner.Graph) error {
	cfg := config.Config{
		Version: 1,
		Project: config.Project{
			Name:        projectNameFromRoot(root),
			Description: fmt.Sprintf("Generado por 'hels scan %s --format hels-yaml'", root),
		},
		Environments: map[string]config.Environment{
			"dev": {
				Engine:    "floci",
				Region:    "us-east-1",
				AccountID: "000000000000",
				Port:      4566,
				Storage:   config.Storage{Mode: config.StorageMemory},
				Services:  scanner.DetectAWSServices(g.Services),
			},
		},
	}

	names := make([]string, len(g.Services))
	for i, s := range g.Services {
		names[i] = s.Name
	}

	fmt.Fprintf(w, "# Generado por 'hels scan %s --format hels-yaml'.\n", root)
	fmt.Fprintf(w, "# Servicios de Serverless Framework encontrados (%d): %s\n", len(names), strings.Join(names, ", "))
	fmt.Fprintf(w, "# Conexiones entre ellos detectadas: %d (correr 'hels scan %s' sin --format para el detalle).\n", len(g.Edges), root)
	fmt.Fprintln(w, "# Este archivo es un punto de partida — revisá region/puerto/servicios antes de usarlo.")
	fmt.Fprintln(w)

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return enc.Close()
}

func projectNameFromRoot(root string) string {
	name := filepath.Base(filepath.Clean(root))
	if name == "." || name == "/" || name == "" {
		return "mi-proyecto"
	}
	return name
}
