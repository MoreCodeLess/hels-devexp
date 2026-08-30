package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFileName)
	if err := os.WriteFile(path, []byte(ExampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Project.Name != "mi-servicio" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "mi-servicio")
	}
	if len(cfg.Environments) != 1 {
		t.Errorf("len(Environments) = %d, want 1", len(cfg.Environments))
	}
}

func TestValidateRejectsUnknownEngine(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Project: Project{Name: "x"},
		Environments: map[string]Environment{
			"dev": {Engine: "not-floci", Storage: Storage{Mode: StorageMemory}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("esperaba error por engine no soportado")
	}
}

func TestValidateRequiresProjectName(t *testing.T) {
	cfg := &Config{
		Version:      1,
		Environments: map[string]Environment{"dev": {Engine: "floci", Storage: Storage{Mode: StorageMemory}}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("esperaba error por project.name vacío")
	}
}

func TestValidateRejectsBadStorageMode(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Project: Project{Name: "x"},
		Environments: map[string]Environment{
			"dev": {Engine: "floci", Storage: Storage{Mode: "not-a-mode"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("esperaba error por storage.mode inválido")
	}
}
