// Package env implementa el ciclo de vida de los entornos locales
// declarados en hels.yaml: levantarlos, bajarlos, consultarlos y cambiar
// cuál es el activo. El motor por ahora es siempre floci, corrido como un
// contenedor Docker directo (sin depender del CLI de floci ni de compose).
package env

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	flociImage = "floci/floci:latest"
	flociPort  = 4566

	labelManaged = "hels.managed"
	labelProject = "hels.project"
	labelEnv     = "hels.environment"
)

// containerName es el nombre del contenedor Docker para un entorno de un
// proyecto dado. Incluye el nombre del proyecto para que dos proyectos
// distintos con un entorno "dev" no choquen entre sí en la misma máquina.
func containerName(project, envName string) string {
	return fmt.Sprintf("hels-%s-%s", project, envName)
}

// containerState es lo que sabemos de un contenedor sin necesidad de que
// exista: si existe y si está corriendo.
type containerState struct {
	Exists  bool
	Running bool
}

func inspectContainer(name string) (containerState, error) {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", name).CombinedOutput()
	if err != nil {
		if isNotFoundError(string(out)) {
			return containerState{}, nil
		}
		return containerState{}, fmt.Errorf("docker inspect %s: %s", name, strings.TrimSpace(string(out)))
	}
	running := strings.TrimSpace(string(out)) == "true"
	return containerState{Exists: true, Running: running}, nil
}

// isNotFoundError detecta si la salida de un comando docker que falló
// significa "el contenedor no existe" en vez de un error real. El texto
// exacto varía entre versiones de Docker ("No such object" vs
// "no such object", con/sin prefijo tipo "Error: No such container: ..."),
// así que se compara sin distinguir mayúsculas en vez de un substring exacto.
func isNotFoundError(output string) bool {
	return strings.Contains(strings.ToLower(output), "no such")
}

func runContainer(args []string) error {
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func removeContainer(name string) error {
	out, err := exec.Command("docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm %s: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}
