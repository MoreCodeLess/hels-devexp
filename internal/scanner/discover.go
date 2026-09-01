package scanner

import (
	"io/fs"
	"path/filepath"
)

// skipDirs son carpetas que nunca tiene sentido escanear: dependencias,
// control de versiones, y el directorio de build de Serverless Framework.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".serverless":  true,
	"vendor":       true,
	".terraform":   true,
}

// Discover recorre root buscando archivos serverless.yml/serverless.yaml,
// sin bajar a carpetas de dependencias/build conocidas.
func Discover(root string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if name := d.Name(); name == "serverless.yml" || name == "serverless.yaml" {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return found, nil
}
