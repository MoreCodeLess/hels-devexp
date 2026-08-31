package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// El panel de logs es un "portal" de tamaño fijo: el alto total que ocupa
// View() tiene que ser siempre exactamente el Height pedido por la
// terminal, sin importar qué tan larga sea una línea de log ni cuántas
// líneas haya. Si esto se rompe, la terminal termina scrolleando el frame
// completo (lista incluida) cada vez que llega contenido largo.
func TestViewHeightIsFixedRegardlessOfContent(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
	}{
		{"sin logs", nil},
		{"lineas cortas", []string{"a", "b", "c"}},
		{"una linea muy larga", []string{strings.Repeat("X", 500)}},
		{"mezcla de cortas y largas", []string{"corta-1", strings.Repeat("Y", 340), "corta-2", strings.Repeat("Z", 12)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
			m.Update(containersLoadedMsg{containers: []Container{{ID: "abc", Name: "svc"}}})
			if tc.lines != nil {
				m.Update(logLinesMsg{gen: m.logGen, lines: tc.lines})
			}

			rows := strings.Split(m.View(), "\n")
			if len(rows) != 25 {
				t.Fatalf("View() devolvió %d filas, quería exactamente 25 (Height pedido) para el caso %q", len(rows), tc.name)
			}
		})
	}
}

func TestHardWrapLine(t *testing.T) {
	short := "hola"
	if got := hardWrapLine(short, 10); len(got) != 1 || got[0] != short {
		t.Fatalf("hardWrapLine(%q, 10) = %v, quería una sola pieza sin cambios", short, got)
	}

	long := strings.Repeat("a", 25)
	got := hardWrapLine(long, 10)
	if len(got) != 3 {
		t.Fatalf("hardWrapLine(25 chars, 10) = %d pedazos, quería 3", len(got))
	}
	if joined := strings.Join(got, ""); joined != long {
		t.Fatalf("hardWrapLine perdió contenido: got %q, want %q", joined, long)
	}
	for i, piece := range got[:len(got)-1] {
		if len([]rune(piece)) != 10 {
			t.Errorf("pedazo %d tiene %d chars, quería 10", i, len([]rune(piece)))
		}
	}
}
