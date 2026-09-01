package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// LambdaFunction es una función Lambda desplegada contra un entorno floci.
type LambdaFunction struct {
	Name string
}

// listLambdaFunctionsResponse es la forma mínima que nos interesa de la
// respuesta de floci a GET /2015-03-31/functions/ (misma forma que la API
// real de Lambda ListFunctions).
type listLambdaFunctionsResponse struct {
	Functions []struct {
		FunctionName string `json:"FunctionName"`
	} `json:"Functions"`
}

// listLambdaFunctions pregunta a floci qué funciones Lambda hay desplegadas.
// No hace falta el AWS CLI ni firmar la petición — floci, igual que para
// invocar, acepta esta llamada como HTTP plano.
func listLambdaFunctions(endpoint string) ([]LambdaFunction, error) {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(strings.TrimRight(endpoint, "/") + "/2015-03-31/functions/")
	if err != nil {
		return nil, fmt.Errorf("consultando funciones Lambda en %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listando funciones Lambda: floci respondió %s", resp.Status)
	}

	var parsed listLambdaFunctionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("leyendo respuesta de funciones Lambda: %w", err)
	}

	fns := make([]LambdaFunction, 0, len(parsed.Functions))
	for _, f := range parsed.Functions {
		fns = append(fns, LambdaFunction{Name: f.FunctionName})
	}
	return fns, nil
}

// listLambdaContainers devuelve TODOS los contenedores efímeros que floci
// tiene corriendo ahora mismo para invocaciones "en caliente" (nombrados
// "floci-<función>-<hash>", ver ContainerLauncher en los logs de floci). Se
// pide una sola vez por refresco y se hace el matching por función en
// memoria (matchLambdaContainer), en vez de un "docker ps" por función — con
// decenas de Lambdas desplegadas eso sería un docker ps por cada una, cada
// pocos segundos.
func listLambdaContainers() ([]Container, error) {
	out, err := exec.Command("docker", "ps", "--filter", "name=^floci-", "--format", psFormat).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("docker ps: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("corriendo docker ps: %w", err)
	}
	return parsePS(string(out)), nil
}

// matchLambdaContainer busca, dentro de containers (ver listLambdaContainers),
// el que floci tiene corriendo ahora para functionName. Puede no encontrar
// nada si la función está "fría" (floci bajó el contenedor por inactividad
// desde la última invocación) — no es un error, solo que no hay nada que
// mostrar hasta la próxima invocación.
func matchLambdaContainer(containers []Container, functionName string) (Container, bool) {
	prefix := "floci-" + functionName + "-"
	for _, c := range containers {
		if strings.HasPrefix(c.Name, prefix) {
			return c, true
		}
	}
	return Container{}, false
}
