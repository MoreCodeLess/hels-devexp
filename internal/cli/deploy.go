package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/MoreCodeLess/hels-devexp/internal/deploy"
	hlenv "github.com/MoreCodeLess/hels-devexp/internal/env"
	"github.com/MoreCodeLess/hels-devexp/internal/scanner"
)

var deployEnvName string

var deployCmd = &cobra.Command{
	Use:   "deploy <ruta>",
	Short: "Crea en floci los recursos (Lambda, SQS, SNS, DynamoDB, S3) que describen los serverless.yml bajo una ruta",
	Long: `Escanea <ruta> (igual que "hels scan") y crea, contra el entorno de
floci indicado, los recursos reales: funciones Lambda (empaquetando el
handler), colas SQS, tópicos SNS, tablas DynamoDB y buckets S3.

No pasa por CloudFormation ("serverless deploy" por dentro) — se probó ese
camino contra floci y tiene un bug real con el handler de las funciones.
Yendo directo a la API de cada servicio se lo evita, y de paso no hace
falta tener Node/npm/serverless-localstack instalados: solo necesita el
AWS CLI.

Recursos que todavía no crea (API Gateway, Cognito, ElastiCache, ECS, ...)
se listan al final como "salteados", no se ignoran en silencio.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := args[0]

		cfg, err := loadProjectConfig()
		if err != nil {
			return err
		}
		envCfg, ok := cfg.Environments[deployEnvName]
		if !ok {
			return fmt.Errorf("el entorno %q no está declarado en hels.yaml", deployEnvName)
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Asegurando que %s esté arriba...\n", deployEnvName)
		st, err := hlenv.Up(cfg, deployEnvName)
		if err != nil {
			return err
		}

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

		fmt.Fprintf(cmd.ErrOrStderr(), "Desplegando %d servicio(s) contra %s...\n", len(services), st.EndpointURL())
		res, err := deploy.Deploy(deploy.Options{Endpoint: st.EndpointURL(), Region: envCfg.Region}, services)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		printDeployList(out, "Tablas DynamoDB", res.Tables)
		printDeployList(out, "Colas SQS", res.Queues)
		printDeployList(out, "Tópicos SNS", res.Topics)
		printDeployList(out, "Buckets S3", res.Buckets)
		printDeployList(out, "Funciones Lambda", res.Functions)
		printDeployList(out, "Salteados (no soportados todavía)", res.Skipped)

		return nil
	},
}

func printDeployList(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", item)
	}
}

func init() {
	deployCmd.Flags().StringVar(&deployEnvName, "env", "dev", "Entorno de hels.yaml contra el que desplegar")
	rootCmd.AddCommand(deployCmd)
}
