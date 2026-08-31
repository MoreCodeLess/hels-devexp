package env

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// StateDir es la carpeta local (no versionada) donde vive el estado de
// ejecución de hels: qué entorno está activo ahora mismo en esta máquina.
// A propósito nunca vive en hels.yaml (eso es config declarativa,
// reproducible y versionada; esto es estado efímero de una sesión de
// trabajo local).
const StateDir = ".hels"

const stateFileName = "state.json"

// State es el estado local persistido.
type State struct {
	// Active es el nombre del entorno activo, o "" si ninguno.
	Active string `json:"active"`
}

func statePath() string {
	return filepath.Join(StateDir, stateFileName)
}

// LoadState lee el estado local. Si todavía no existe, devuelve un State
// vacío (sin error) — es la situación normal antes del primer "env switch".
func LoadState() (*State, error) {
	data, err := os.ReadFile(statePath())
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveState persiste el estado local, creando StateDir si hace falta.
func SaveState(s *State) error {
	if err := os.MkdirAll(StateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), data, 0o644)
}
