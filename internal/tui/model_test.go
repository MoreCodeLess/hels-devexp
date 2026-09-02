package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
)

func TestHitTestList(t *testing.T) {
	m := &Model{
		categoryFilter: kindAll,
		viewport:       viewport.New(80, 10), // suficiente para que entren los 3 ítems (2 filas c/u)
		items: []listItem{
			{kind: kindProcess, key: "proc:gateway", name: "gateway"},
			{kind: kindContainer, key: "b", name: "svc-b"},
			{kind: kindContainer, key: "c", name: "svc-c"},
		},
	}

	tests := []struct {
		name    string
		x, y    int
		wantIdx int
		wantOK  bool
	}{
		{"primer ítem, fila del nombre", 5, listItemsTopRow, 0, true},
		{"primer ítem, fila del estado", 5, listItemsTopRow + 1, 0, true},
		{"segundo ítem, fila del nombre", 5, listItemsTopRow + 2, 1, true},
		{"segundo ítem, fila del estado", 5, listItemsTopRow + 3, 1, true},
		{"tercer ítem", 5, listItemsTopRow + 4, 2, true},
		{"fila arriba del primer ítem (título)", 5, listItemsTopRow - 1, 0, false},
		{"fila debajo del último ítem", 5, listItemsTopRow + 6, 0, false},
		{"fuera del panel de lista (a la derecha)", listPaneOuterWidth, listItemsTopRow, 0, false},
		{"x negativo", -1, listItemsTopRow, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, ok := m.hitTestList(tt.x, tt.y)
			if ok != tt.wantOK {
				t.Fatalf("hitTestList(%d,%d) ok = %v, want %v", tt.x, tt.y, ok, tt.wantOK)
			}
			if ok && idx != tt.wantIdx {
				t.Errorf("hitTestList(%d,%d) idx = %d, want %d", tt.x, tt.y, idx, tt.wantIdx)
			}
		})
	}
}

// TestHitTestCategoryTabs cubre la matemática de columnas del menú de
// categorías clickeable — con proceso + contenedor presentes, el layout es
// "[*] [>] [#]" (kindAll, kindProcess, kindContainer), cada pestaña de 3
// columnas separadas por 1 espacio.
func TestHitTestCategoryTabs(t *testing.T) {
	m := &Model{
		categoryFilter: kindAll,
		items: []listItem{
			{kind: kindProcess, key: "proc:gateway", name: "gateway"},
			{kind: kindContainer, key: "b", name: "svc-b"},
		},
	}

	tests := []struct {
		name     string
		x, y     int
		wantKind itemKind
		wantOK   bool
	}{
		{"fila equivocada", 2, categoryTabsRow + 1, kindAll, false},
		{"pestaña 'todos', primera columna", 2, categoryTabsRow, kindAll, true},
		{"pestaña 'todos', última columna", 4, categoryTabsRow, kindAll, true},
		{"espacio entre pestañas", 5, categoryTabsRow, kindAll, false},
		{"pestaña procesos", 6, categoryTabsRow, kindProcess, true},
		{"pestaña contenedores", 10, categoryTabsRow, kindContainer, true},
		{"después de la última pestaña", 13, categoryTabsRow, kindAll, false},
		{"x negativo", -1, categoryTabsRow, kindAll, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, ok := m.hitTestCategoryTabs(tt.x, tt.y)
			if ok != tt.wantOK {
				t.Fatalf("hitTestCategoryTabs(%d,%d) ok = %v, want %v", tt.x, tt.y, ok, tt.wantOK)
			}
			if ok && k != tt.wantKind {
				t.Errorf("hitTestCategoryTabs(%d,%d) kind = %v, want %v", tt.x, tt.y, k, tt.wantKind)
			}
		})
	}
}
