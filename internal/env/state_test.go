package env

import "testing"

func TestLoadStateWhenMissingReturnsEmpty(t *testing.T) {
	t.Chdir(t.TempDir())

	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if s.Active != "" {
		t.Errorf("LoadState() sin archivo previo = %+v, quería Active vacío", s)
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := SaveState(&State{Active: "dev"}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.Active != "dev" {
		t.Errorf("LoadState() = %+v, quería Active = \"dev\"", got)
	}
}
