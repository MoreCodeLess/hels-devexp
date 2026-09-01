package env

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
)

// Status es el estado observable de un entorno, para mostrar en
// `hels env status`/`list` o para armar las variables de conexión en
// `hels env switch`.
type Status struct {
	Name      string
	Container string
	Running   bool
	Port      int
	Region    string
	AccountID string
	Storage   config.StorageMode
}

// EndpointURL es la URL que hay que apuntar en AWS_ENDPOINT_URL para hablar
// con este entorno.
func (s Status) EndpointURL() string {
	return fmt.Sprintf("http://localhost:%d", s.Port)
}

// Up levanta (o confirma que ya está corriendo) el entorno envName del
// proyecto cfg. Es idempotente: si el contenedor ya existe y está
// corriendo, no hace nada y devuelve su estado tal cual.
func Up(cfg *config.Config, envName string) (*Status, error) {
	envCfg, ok := cfg.Environments[envName]
	if !ok {
		return nil, fmt.Errorf("el entorno %q no está declarado en %s", envName, config.DefaultFileName)
	}

	name := containerName(cfg.Project.Name, envName)

	state, err := inspectContainer(name)
	if err != nil {
		return nil, err
	}

	if state.Exists && !state.Running {
		// Quedó de una corrida anterior (ej. Docker Desktop se reinició) —
		// se recrea limpio en vez de intentar arrancar un contenedor viejo.
		if err := removeContainer(name); err != nil {
			return nil, err
		}
		state.Exists = false
	}

	if !state.Exists {
		args, err := runArgs(cfg.Project.Name, envName, envCfg)
		if err != nil {
			return nil, err
		}
		if err := runContainer(args); err != nil {
			return nil, err
		}
	}

	return &Status{
		Name:      envName,
		Container: name,
		Running:   true,
		Port:      envCfg.Port,
		Region:    envCfg.Region,
		AccountID: envCfg.AccountID,
		Storage:   envCfg.Storage.Mode,
	}, nil
}

// runArgs arma los argumentos de "docker run" para un entorno. Separado de
// Up para poder testearlo sin necesitar Docker instalado.
func runArgs(project, envName string, envCfg config.Environment) ([]string, error) {
	args := []string{
		"run", "-d",
		"--name", containerName(project, envName),
		"--label", labelManaged + "=true",
		"--label", labelProject + "=" + project,
		"--label", labelEnv + "=" + envName,
		"-p", fmt.Sprintf("%d:%d", envCfg.Port, flociPort),
		// Docker-outside-of-Docker: floci corre Lambda, ECS, RDS, ElastiCache,
		// etc. como contenedores Docker reales (no los simula en proceso), así
		// que necesita hablar con el Docker del host para poder levantarlos.
		// Sin este mount, "create-function" funciona (floci solo guarda
		// metadata) pero "invoke" falla al intentar arrancar el contenedor del
		// runtime — confirmado en pruebas reales contra el proyecto TaskFlow.
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", "FLOCI_DEFAULT_REGION=" + envCfg.Region,
		"-e", "FLOCI_DEFAULT_ACCOUNT_ID=" + envCfg.AccountID,
		"-e", "FLOCI_STORAGE_MODE=" + string(envCfg.Storage.Mode),
	}

	if envCfg.Storage.Mode != config.StorageMemory {
		if envCfg.Storage.Path == "" {
			return nil, fmt.Errorf("entorno %q: storage.mode %q necesita storage.path", envName, envCfg.Storage.Mode)
		}
		hostPath, err := filepath.Abs(envCfg.Storage.Path)
		if err != nil {
			return nil, fmt.Errorf("resolviendo storage.path de %q: %w", envName, err)
		}
		if err := os.MkdirAll(hostPath, 0o755); err != nil {
			return nil, fmt.Errorf("creando storage.path de %q: %w", envName, err)
		}
		args = append(args,
			"-e", "FLOCI_STORAGE_PERSISTENT_PATH=/data",
			"-v", hostPath+":/data",
		)
	}

	args = append(args, flociImage)
	return args, nil
}

// Down baja el entorno envName. No es un error bajar un entorno que ya
// estaba abajo.
func Down(cfg *config.Config, envName string) error {
	if _, ok := cfg.Environments[envName]; !ok {
		return fmt.Errorf("el entorno %q no está declarado en %s", envName, config.DefaultFileName)
	}

	name := containerName(cfg.Project.Name, envName)
	state, err := inspectContainer(name)
	if err != nil {
		return err
	}
	if !state.Exists {
		return nil
	}
	return removeContainer(name)
}

// StatusOf consulta el estado actual de un entorno.
func StatusOf(cfg *config.Config, envName string) (*Status, error) {
	envCfg, ok := cfg.Environments[envName]
	if !ok {
		return nil, fmt.Errorf("el entorno %q no está declarado en %s", envName, config.DefaultFileName)
	}

	name := containerName(cfg.Project.Name, envName)
	state, err := inspectContainer(name)
	if err != nil {
		return nil, err
	}

	return &Status{
		Name:      envName,
		Container: name,
		Running:   state.Running,
		Port:      envCfg.Port,
		Region:    envCfg.Region,
		AccountID: envCfg.AccountID,
		Storage:   envCfg.Storage.Mode,
	}, nil
}

// List consulta el estado de todos los entornos declarados en cfg, en un
// orden estable (alfabético por nombre).
func List(cfg *config.Config) ([]*Status, error) {
	names := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		names = append(names, name)
	}
	sort.Strings(names)

	statuses := make([]*Status, 0, len(names))
	for _, name := range names {
		st, err := StatusOf(cfg, name)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, st)
	}
	return statuses, nil
}
