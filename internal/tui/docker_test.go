package tui

import (
	"reflect"
	"testing"
)

func TestParsePS(t *testing.T) {
	input := "abc123\tssh-dev\tUp 5 minutes\r\n" +
		"def456\tkrakend\tUp 2 hours\r\n" +
		"\n"

	got := parsePS(input)
	want := []Container{
		{ID: "abc123", Name: "ssh-dev", Status: "Up 5 minutes"},
		{ID: "def456", Name: "krakend", Status: "Up 2 hours"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePS() = %+v, want %+v", got, want)
	}
}

func TestParsePSEmpty(t *testing.T) {
	if got := parsePS(""); got != nil {
		t.Errorf("parsePS(\"\") = %+v, want nil", got)
	}
}

func TestParsePSIgnoresMalformedLines(t *testing.T) {
	got := parsePS("solo-un-campo\n")
	if len(got) != 0 {
		t.Errorf("parsePS() = %+v, want vacío para línea sin 3 campos", got)
	}
}
