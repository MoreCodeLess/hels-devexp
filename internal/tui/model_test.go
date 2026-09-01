package tui

import "testing"

func TestHitTestList(t *testing.T) {
	m := &Model{
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
