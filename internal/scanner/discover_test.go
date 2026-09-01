package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverFindsServerlessFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "orders", "serverless.yml"), "service: orders\n")
	writeFile(t, filepath.Join(dir, "shipping", "serverless.yaml"), "service: shipping\n")
	writeFile(t, filepath.Join(dir, "shipping", "node_modules", "some-dep", "serverless.yml"), "service: deberia-ignorarse\n")
	writeFile(t, filepath.Join(dir, "shipping", "README.md"), "# no es un serverless.yml\n")

	found, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	var names []string
	for _, f := range found {
		names = append(names, filepath.Base(filepath.Dir(f))+"/"+filepath.Base(f))
	}
	sort.Strings(names)

	want := []string{"orders/serverless.yml", "shipping/serverless.yaml"}
	if len(names) != len(want) {
		t.Fatalf("Discover() encontró %v, quería %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("Discover()[%d] = %q, quería %q", i, names[i], want[i])
		}
	}
}
