package tui

import "testing"

func TestHitTestList(t *testing.T) {
	m := &Model{
		containers: []Container{
			{ID: "a", Name: "svc-a"},
			{ID: "b", Name: "svc-b"},
			{ID: "c", Name: "svc-c"},
		},
	}

	tests := []struct {
		name    string
		x, y    int
		wantIdx int
		wantOK  bool
	}{
		{"primer ítem", 5, listItemsTopRow, 0, true},
		{"segundo ítem", 5, listItemsTopRow + 1, 1, true},
		{"tercer ítem", 5, listItemsTopRow + 2, 2, true},
		{"fila arriba del primer ítem (título)", 5, listItemsTopRow - 1, 0, false},
		{"fila debajo del último ítem", 5, listItemsTopRow + 3, 0, false},
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
