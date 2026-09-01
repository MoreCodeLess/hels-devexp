package tui

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"syscall"
)

// ProcessSpec es un proceso local declarado en hels.yaml (processes.*): un
// gateway, un frontend, un worker, lo que sea. hels no sabe qué hace el
// comando — lo corre tal cual y muestra su salida, como mprocs.
type ProcessSpec struct {
	Name string
	Cmd  string
	Dir  string
}

// procHandle es un proceso local corriendo, con su salida combinada
// (stdout+stderr) disponible línea por línea a través de Lines. A diferencia
// de un stream de logs de Docker, un proceso arranca UNA vez al iniciar el
// dashboard y sigue corriendo en segundo plano sin importar qué esté
// seleccionado en la lista — su buffer de líneas vive en procEntry, no acá.
type procHandle struct {
	cmd   *exec.Cmd
	Lines chan string
	Done  chan error
}

// startProcess corre cmdStr con "sh -c" (dir como working directory si se
// especifica) y va emitiendo su salida combinada línea por línea por Lines.
// El proceso corre en su propio grupo de procesos (Setpgid) para poder
// matar también a los hijos que arranque (ej. "npm run dev" que a su vez
// levanta node) al pararlo.
func startProcess(cmdStr, dir string) (*procHandle, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	h := &procHandle{
		cmd:   cmd,
		Lines: make(chan string, 256),
		Done:  make(chan error, 1),
	}

	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			h.Lines <- scanner.Text()
		}
		close(h.Lines)
	}()

	go func() {
		err := cmd.Wait()
		pw.Close()
		h.Done <- err
	}()

	return h, nil
}

// Stop mata el proceso completo (su grupo de procesos, no solo el "sh"
// inicial) con SIGTERM. Seguro de llamar más de una vez.
func (h *procHandle) Stop() {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(h.cmd.Process.Pid)
	if err != nil {
		_ = h.cmd.Process.Signal(os.Interrupt)
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
}
