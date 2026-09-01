package env

import (
	"strings"
	"testing"

	"github.com/MoreCodeLess/hels-devexp/internal/config"
)

func TestContainerName(t *testing.T) {
	got := containerName("mi-servicio", "dev")
	want := "hels-mi-servicio-dev"
	if got != want {
		t.Errorf("containerName() = %q, want %q", got, want)
	}
}

func TestRunArgsMemoryStorage(t *testing.T) {
	envCfg := config.Environment{
		Region:    "us-east-1",
		AccountID: "000000000000",
		Port:      4566,
		Storage:   config.Storage{Mode: config.StorageMemory},
	}

	args, err := runArgs("mi-servicio", "dev", envCfg)
	if err != nil {
		t.Fatalf("runArgs() error = %v", err)
	}

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--name hels-mi-servicio-dev",
		"--label hels.managed=true",
		"--label hels.project=mi-servicio",
		"--label hels.environment=dev",
		"-p 4566:4566",
		"-v /var/run/docker.sock:/var/run/docker.sock",
		"-e FLOCI_DEFAULT_REGION=us-east-1",
		"-e FLOCI_DEFAULT_ACCOUNT_ID=000000000000",
		"-e FLOCI_STORAGE_MODE=memory",
		"floci/floci:latest",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("runArgs() = %q, esperaba que contuviera %q", joined, want)
		}
	}
	if strings.Contains(joined, "FLOCI_STORAGE_PERSISTENT_PATH") {
		t.Errorf("runArgs() con storage memory no debería montar un volumen: %q", joined)
	}
}

func TestRunArgsPersistentStorageRequiresPath(t *testing.T) {
	envCfg := config.Environment{
		Storage: config.Storage{Mode: config.StoragePersistent}, // sin Path
	}

	if _, err := runArgs("mi-servicio", "dev", envCfg); err == nil {
		t.Fatal("runArgs() con storage persistent sin path debería fallar")
	}
}

func TestRunArgsPersistentStorageMountsVolume(t *testing.T) {
	dir := t.TempDir()
	envCfg := config.Environment{
		Storage: config.Storage{Mode: config.StoragePersistent, Path: dir},
	}

	args, err := runArgs("mi-servicio", "dev", envCfg)
	if err != nil {
		t.Fatalf("runArgs() error = %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "FLOCI_STORAGE_PERSISTENT_PATH=/data") {
		t.Errorf("runArgs() con storage persistent debería setear FLOCI_STORAGE_PERSISTENT_PATH: %q", joined)
	}
	if !strings.Contains(joined, "-v "+dir+":/data") {
		t.Errorf("runArgs() con storage persistent debería montar -v %s:/data: %q", dir, joined)
	}
}
