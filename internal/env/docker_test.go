package env

import "testing"

func TestIsNotFoundError(t *testing.T) {
	cases := []struct {
		output string
		want   bool
	}{
		{"Error: No such object: hels-mi-servicio-dev", true},
		{"error: no such object: hels-mi-servicio-dev", true},
		{"Error: No such container: hels-mi-servicio-dev", true},
		{"permission denied while trying to connect to the Docker daemon socket", false},
		{"Cannot connect to the Docker daemon at unix:///var/run/docker.sock", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := isNotFoundError(tc.output); got != tc.want {
			t.Errorf("isNotFoundError(%q) = %v, want %v", tc.output, got, tc.want)
		}
	}
}
