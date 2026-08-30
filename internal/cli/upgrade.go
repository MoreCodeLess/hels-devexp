package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const upgradeRepo = "MoreCodeLess/hels-devexp"

var (
	upgradeVersion string
	upgradeForce   bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Descarga e instala la última versión de hels, reemplazando el binario actual",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			return fmt.Errorf("hels upgrade no soporta esta plataforma: %s", runtime.GOOS)
		}

		version := upgradeVersion
		if version == "" {
			tag, err := latestReleaseTag(upgradeRepo)
			if err != nil {
				return fmt.Errorf("consultando última versión: %w", err)
			}
			version = tag
		}

		if !upgradeForce && version == "v"+strings.TrimPrefix(Version, "v") {
			fmt.Fprintf(cmd.OutOrStdout(), "Ya estás en la última versión (%s)\n", Version)
			return nil
		}

		asset := assetName(runtime.GOOS, runtime.GOARCH)
		url := downloadURL(upgradeRepo, version, asset)

		fmt.Fprintf(cmd.OutOrStdout(), "Descargando %s (%s)...\n", asset, version)
		binData, err := fetchBinaryFromTarGz(url, "hels")
		if err != nil {
			return err
		}

		if err := replaceRunningBinary(binData); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "hels actualizado a %s\n", version)
		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "", "Tag específico a instalar, ej. v0.1.0 (default: el último release)")
	upgradeCmd.Flags().BoolVar(&upgradeForce, "force", false, "Reinstala aunque ya estés en la última versión")
}

func assetName(osName, arch string) string {
	return fmt.Sprintf("hels_%s_%s.tar.gz", osName, arch)
}

func downloadURL(repo, version, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, asset)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func latestReleaseTag(repo string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API respondió %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// fetchBinaryFromTarGz descarga un .tar.gz y devuelve el contenido del archivo
// binName que encuentre adentro.
func fetchBinaryFromTarGz(url, binName string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("descargando release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("descargando release: HTTP %d (%s)", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("abriendo tar.gz: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("leyendo tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binName {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("leyendo binario del tar: %w", err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("no se encontró el binario %q dentro del archivo descargado", binName)
}

// replaceRunningBinary sobreescribe el ejecutable actual con binData, escribiendo
// primero a un archivo temporal en el mismo directorio y haciendo rename atómico
// (seguro incluso mientras el binario actual sigue corriendo en Linux/macOS).
func replaceRunningBinary(binData []byte) error {
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolviendo binario actual: %w", err)
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		return fmt.Errorf("resolviendo symlinks de %s: %w", currentPath, err)
	}

	dir := filepath.Dir(currentPath)
	tmpFile, err := os.CreateTemp(dir, ".hels-upgrade-*")
	if err != nil {
		return fmt.Errorf("creando archivo temporal en %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // no-op si el rename ya lo movió

	if _, err := tmpFile.Write(binData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("escribiendo binario nuevo: %w", err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		return fmt.Errorf("reemplazando %s: %w", currentPath, err)
	}
	return nil
}
