package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MoreCodeLess/hels-devexp/internal/scanner"
)

const (
	execRoleName = "hels-lambda-exec-role"
	execRoleARN  = "arn:aws:iam::000000000000:role/" + execRoleName

	assumeRolePolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`
)

// Options son los datos de conexión al floci contra el que se va a desplegar.
type Options struct {
	Endpoint string // ej. http://localhost:4566
	Region   string
}

// Result resume lo que se creó (o se saltó) en una corrida de Deploy.
type Result struct {
	Tables    []string
	Queues    []string
	Topics    []string
	Buckets   []string
	Functions []string
	// Skipped son recursos que hels deploy todavía no sabe crear (ej. API
	// Gateway, Cognito, ElastiCache, ECS) — se listan para que sea explícito
	// qué falta, no se ignoran en silencio.
	Skipped []string
}

// Deploy crea, contra el floci descripto por opts, los recursos de todos los
// services. No pasa por CloudFormation — ver el comentario de paquete.
func Deploy(opts Options, services []*scanner.ServiceDef) (*Result, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return nil, fmt.Errorf("hels deploy necesita el AWS CLI instalado (no reimplementamos la API de AWS): %w", err)
	}

	res := &Result{}

	if err := ensureExecRole(opts); err != nil {
		return nil, fmt.Errorf("creando el rol de ejecución de Lambda: %w", err)
	}

	for _, svc := range services {
		for _, r := range svc.Resources {
			switch r.Type {
			case "AWS::DynamoDB::Table":
				if err := createTable(opts, r); err != nil {
					return res, fmt.Errorf("%s: tabla %s: %w", svc.Name, r.LogicalID, err)
				}
				res.Tables = append(res.Tables, nonEmpty(r.Name, r.LogicalID))

			case "AWS::SQS::Queue":
				if err := createQueue(opts, r); err != nil {
					return res, fmt.Errorf("%s: cola %s: %w", svc.Name, r.LogicalID, err)
				}
				res.Queues = append(res.Queues, nonEmpty(r.Name, r.LogicalID))

			case "AWS::SNS::Topic":
				if err := createTopic(opts, r); err != nil {
					return res, fmt.Errorf("%s: tópico %s: %w", svc.Name, r.LogicalID, err)
				}
				res.Topics = append(res.Topics, nonEmpty(r.Name, r.LogicalID))

			case "AWS::S3::Bucket":
				if err := createBucket(opts, r); err != nil {
					return res, fmt.Errorf("%s: bucket %s: %w", svc.Name, r.LogicalID, err)
				}
				res.Buckets = append(res.Buckets, nonEmpty(r.Name, r.LogicalID))

			default:
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s/%s (%s)", svc.Name, r.LogicalID, r.Type))
			}
		}

		serviceDir := filepath.Dir(svc.Path)
		for _, fn := range svc.Functions {
			if fn.Handler == "" {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s/%s (función sin handler)", svc.Name, fn.Name))
				continue
			}
			if !strings.HasPrefix(svc.Runtime, "nodejs") {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s/%s (runtime %q no soportado por hels deploy todavía)", svc.Name, fn.Name, svc.Runtime))
				continue
			}
			if err := deployFunction(opts, serviceDir, svc.Runtime, fn); err != nil {
				return res, fmt.Errorf("%s: función %s: %w", svc.Name, fn.Name, err)
			}
			res.Functions = append(res.Functions, fn.Name)
		}
	}

	return res, nil
}

func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func awsCLI(opts Options, args ...string) *exec.Cmd {
	full := append([]string{"--endpoint-url", opts.Endpoint, "--region", opts.Region}, args...)
	cmd := exec.Command("aws", full...)
	cmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID=test",
		"AWS_SECRET_ACCESS_KEY=test",
	)
	return cmd
}

// alreadyExists detecta si la salida de un comando fallido significa "ya
// existe" en vez de un error real — para que correr "hels deploy" dos veces
// no rompa nada.
func alreadyExists(output string) bool {
	lower := strings.ToLower(output)
	for _, marker := range []string{"already exist", "entityalreadyexists", "resourceinuseexception", "bucketalreadyowned"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func ensureExecRole(opts Options) error {
	out, err := awsCLI(opts, "iam", "create-role",
		"--role-name", execRoleName,
		"--assume-role-policy-document", assumeRolePolicy,
	).CombinedOutput()
	if err != nil && !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func createTable(opts Options, r scanner.Resource) error {
	props := map[string]interface{}{}
	for k, v := range r.Properties {
		if k == "StreamSpecification" {
			continue // no hace falta para que la tabla exista, y simplifica el CLI input
		}
		props[k] = v
	}
	if _, ok := props["TableName"]; !ok {
		props["TableName"] = r.Name
	}

	body, err := json.Marshal(props)
	if err != nil {
		return err
	}

	out, err := awsCLI(opts, "dynamodb", "create-table", "--cli-input-json", string(body)).CombinedOutput()
	if err != nil && !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func createQueue(opts Options, r scanner.Resource) error {
	out, err := awsCLI(opts, "sqs", "create-queue", "--queue-name", r.Name).CombinedOutput()
	if err != nil && !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func createTopic(opts Options, r scanner.Resource) error {
	out, err := awsCLI(opts, "sns", "create-topic", "--name", r.Name).CombinedOutput()
	if err != nil && !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func createBucket(opts Options, r scanner.Resource) error {
	out, err := awsCLI(opts, "s3api", "create-bucket", "--bucket", r.Name).CombinedOutput()
	if err != nil && !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func deployFunction(opts Options, serviceDir, runtime string, fn scanner.Function) error {
	zipPath, flatHandler, err := buildFunctionZip(serviceDir, fn.Handler)
	if err != nil {
		return err
	}
	defer os.Remove(zipPath)

	out, err := awsCLI(opts, "lambda", "create-function",
		"--function-name", fn.Name,
		"--runtime", runtime,
		"--role", execRoleARN,
		"--handler", flatHandler,
		"--zip-file", "fileb://"+zipPath,
	).CombinedOutput()
	if err == nil {
		return nil
	}
	if !alreadyExists(string(out)) {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	// Ya existe: actualizamos el código en vez de fallar, para que
	// "hels deploy" se pueda correr de nuevo después de cambiar el handler.
	out, err = awsCLI(opts, "lambda", "update-function-code",
		"--function-name", fn.Name,
		"--zip-file", "fileb://"+zipPath,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
