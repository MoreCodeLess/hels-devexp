package deploy

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildFunctionZipFlattensNestedHandler(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "exports.createTask = async () => ({ statusCode: 201 });"
	if err := os.WriteFile(filepath.Join(srcDir, "handler.js"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, flatHandler, err := buildFunctionZip(dir, "src/handler.createTask")
	if err != nil {
		t.Fatalf("buildFunctionZip() error = %v", err)
	}
	defer os.Remove(zipPath)

	if flatHandler != "handler.createTask" {
		t.Errorf("flatHandler = %q, want %q", flatHandler, "handler.createTask")
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("abriendo el zip generado: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("el zip tiene %d archivos, quería 1", len(r.File))
	}
	if r.File[0].Name != "handler.js" {
		t.Errorf("el archivo en el zip se llama %q, quería %q (en la raíz, sin 'src/')", r.File[0].Name, "handler.js")
	}

	rc, err := r.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != content {
		t.Errorf("contenido del handler en el zip = %q, want %q", buf, content)
	}
}

func TestBuildFunctionZipInvalidHandler(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := buildFunctionZip(dir, "sin-punto"); err == nil {
		t.Fatal("buildFunctionZip() con un handler sin '.' debería fallar")
	}
}

func TestAlreadyExists(t *testing.T) {
	cases := map[string]bool{
		"An error occurred (ResourceInUseException) when calling the CreateTable operation": true,
		"An error occurred (EntityAlreadyExists) when calling the CreateRole operation":       true,
		"BucketAlreadyOwnedByYou":                                                             true,
		"An error occurred (AccessDenied) when calling the CreateFunction operation":          false,
		"":                                                                                     false,
	}
	for output, want := range cases {
		if got := alreadyExists(output); got != want {
			t.Errorf("alreadyExists(%q) = %v, want %v", output, got, want)
		}
	}
}
