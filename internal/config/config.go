// Package config define el esquema de hels.yaml: la única fuente de verdad
// para los entornos locales que orquesta hels.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultFileName es el nombre de archivo que hels busca en el directorio actual.
const DefaultFileName = "hels.yaml"

// StorageMode controla cómo persiste floci los datos de un entorno.
type StorageMode string

const (
	StorageMemory     StorageMode = "memory"
	StoragePersistent StorageMode = "persistent"
	StorageHybrid     StorageMode = "hybrid"
	StorageWAL        StorageMode = "wal"
)

// Storage configura la persistencia de un entorno.
type Storage struct {
	Mode StorageMode `yaml:"mode"`
	Path string      `yaml:"path,omitempty"`
}

// Environment describe un entorno local simulado (ej. dev, staging).
type Environment struct {
	Engine    string   `yaml:"engine"` // motor de simulación; hoy solo "floci"
	Region    string   `yaml:"region"`
	AccountID string   `yaml:"account_id"`
	Port      int      `yaml:"port"`
	Storage   Storage  `yaml:"storage"`
	Services  []string `yaml:"services"`
}

// Project describe metadata del proyecto/repo dueño de este hels.yaml.
type Project struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// Config es el esquema raíz de hels.yaml.
type Config struct {
	Version      int                    `yaml:"version"`
	Project      Project                `yaml:"project"`
	Environments map[string]Environment `yaml:"environments"`
}

// Load lee, parsea y valida un hels.yaml desde disco.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validando %s: %w", path, err)
	}

	return &cfg, nil
}

// Validate revisa que el config tenga la forma mínima esperada.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("version %d no soportada (esperada: 1)", c.Version)
	}
	if c.Project.Name == "" {
		return fmt.Errorf("project.name es obligatorio")
	}
	if len(c.Environments) == 0 {
		return fmt.Errorf("hay que declarar al menos un entorno en 'environments'")
	}
	for name, env := range c.Environments {
		if env.Engine != "floci" {
			return fmt.Errorf("entorno %q: engine %q no soportado (por ahora solo 'floci')", name, env.Engine)
		}
		switch env.Storage.Mode {
		case StorageMemory, StoragePersistent, StorageHybrid, StorageWAL:
		default:
			return fmt.Errorf("entorno %q: storage.mode %q inválido", name, env.Storage.Mode)
		}
	}
	return nil
}
