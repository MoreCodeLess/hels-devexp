// Package tui implementa el dashboard interactivo de hels (hels dashboard):
// lista de servicios (hoy, contenedores Docker) + logs en vivo del seleccionado.
package tui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Container es un servicio local visible en el dashboard.
type Container struct {
	ID     string
	Name   string
	Status string
}

const psFormat = "{{.ID}}\t{{.Names}}\t{{.Status}}"

// listContainers corre "docker ps" y devuelve los contenedores en ejecución.
func listContainers() ([]Container, error) {
	out, err := exec.Command("docker", "ps", "--format", psFormat).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("docker ps: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("corriendo docker ps: %w", err)
	}
	return parsePS(string(out)), nil
}

// parsePS parsea la salida de "docker ps --format {{.ID}}\t{{.Names}}\t{{.Status}}".
// Separado de listContainers para poder testearlo sin tener Docker instalado.
func parsePS(output string) []Container {
	var containers []Container
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		containers = append(containers, Container{
			ID:     fields[0],
			Name:   fields[1],
			Status: fields[2],
		})
	}
	return containers
}

// logStream es un "docker logs -f" corriendo para un contenedor, con su salida
// combinada (stdout+stderr) disponible línea por línea a través de Lines.
type logStream struct {
	cmd   *exec.Cmd
	Lines chan string
	Done  chan error
}

// startLogStream arranca "docker logs -f --tail N <id>" y va emitiendo cada
// línea por el canal Lines a medida que llega.
func startLogStream(id string, tail int) (*logStream, error) {
	cmd := exec.Command("docker", "logs", "-f", "--tail", fmt.Sprintf("%d", tail), id)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("arrancando docker logs: %w", err)
	}

	ls := &logStream{
		cmd:   cmd,
		Lines: make(chan string, 256),
		Done:  make(chan error, 1),
	}

	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			ls.Lines <- scanner.Text()
		}
		close(ls.Lines)
	}()

	go func() {
		err := cmd.Wait()
		pw.Close()
		ls.Done <- err
	}()

	return ls, nil
}

// Stop mata el proceso "docker logs -f" subyacente. Es seguro llamarlo más de
// una vez y no bloquea esperando a que el proceso termine de limpiarse.
func (ls *logStream) Stop() {
	if ls == nil || ls.cmd == nil || ls.cmd.Process == nil {
		return
	}
	_ = ls.cmd.Process.Kill()
}
