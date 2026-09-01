package cli

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
	"github.com/MoreCodeLess/hels-devexp/internal/scanner"
)

func TestProjectNameFromRoot(t *testing.T) {
	cases := map[string]string{
		"./mi-monorepo":     "mi-monorepo",
		"/home/dev/repo":    "repo",
		"repo/":             "repo",
		".":                 "mi-proyecto",
		"":                  "mi-proyecto",
	}
	for root, want := range cases {
		if got := projectNameFromRoot(root); got != want {
			t.Errorf("projectNameFromRoot(%q) = %q, want %q", root, got, want)
		}
	}
}

func TestRenderHelsYAMLProducesLoadableConfig(t *testing.T) {
	g := &scanner.Graph{
		Services: []*scanner.ServiceDef{
			{
				Name: "tasks",
				Functions: []scanner.Function{
					{Name: "createTask", Events: []scanner.Event{{Type: "http"}, {Type: "sns"}}},
				},
				Resources: []scanner.Resource{
					{LogicalID: "TasksTable", Type: "AWS::DynamoDB::Table"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := renderHelsYAML(&buf, "./tasks-repo", g); err != nil {
		t.Fatalf("renderHelsYAML() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# Generado por 'hels scan ./tasks-repo") {
		t.Errorf("renderHelsYAML() no incluyó el comentario de origen: %q", out)
	}

	var cfg config.Config
	if err := yaml.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("el hels.yaml generado no es válido: %v\n---\n%s", err, out)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("el hels.yaml generado no pasa Validate(): %v\n---\n%s", err, out)
	}

	env, ok := cfg.Environments["dev"]
	if !ok {
		t.Fatalf("el hels.yaml generado no tiene un entorno 'dev'\n---\n%s", out)
	}
	if env.Engine != "floci" {
		t.Errorf("env.Engine = %q, want %q", env.Engine, "floci")
	}

	wantServices := []string{"apigateway", "dynamodb", "lambda", "sns"}
	if len(env.Services) != len(wantServices) {
		t.Fatalf("env.Services = %v, want %v", env.Services, wantServices)
	}
	for i, s := range wantServices {
		if env.Services[i] != s {
			t.Errorf("env.Services[%d] = %q, want %q", i, env.Services[i], s)
		}
	}
}
