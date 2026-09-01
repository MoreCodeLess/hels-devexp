// Package deploy crea, contra un floci en corrida, los recursos AWS que
// describen los serverless.yml escaneados (internal/scanner): funciones
// Lambda, colas SQS, tópicos SNS, tablas DynamoDB y buckets S3.
//
// A propósito NO pasa por CloudFormation (que es lo que hace "serverless
// deploy" por dentro): se probó ese camino contra floci y su traducción de
// CloudFormation a Lambda tiene un bug real (falla con "Handler file ...
// not found" incluso con el paquete armado correctamente) — ver
// 06-Sessions/2026-08-31-fase3-deploy.md en el vault. Yendo directo a la API
// de cada servicio (vía el AWS CLI, igual que internal/env habla con
// Docker) se evita ese problema por completo, y de paso no hace falta tener
// Node.js/npm/serverless-localstack instalado para usar "hels deploy".
package deploy

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildFunctionZip arma un .zip con SOLO el archivo del handler, puesto en
// la RAÍZ del zip (sin importar en qué subcarpeta viva dentro del servicio,
// ej. "src/handler.js"). Devuelve la ruta al zip temporal y el handler ya
// "aplanado" para pasarle a Lambda (ej. "handler.createTask").
//
// Se aplana a propósito: se confirmó que floci sí puede correr un handler en
// la raíz del paquete, pero no se confirmó que soporte handlers en
// subcarpetas via la API directa — más vale no arriesgar con algo no
// probado.
func buildFunctionZip(serviceDir, handler string) (zipPath, flatHandler string, err error) {
	dotIdx := strings.LastIndex(handler, ".")
	if dotIdx < 0 {
		return "", "", fmt.Errorf("handler %q no tiene la forma <archivo>.<funcion>", handler)
	}
	relFile := handler[:dotIdx] + ".js" // hels deploy solo soporta runtimes nodejs por ahora
	exportName := handler[dotIdx+1:]

	srcPath := filepath.Join(serviceDir, relFile)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", "", fmt.Errorf("leyendo %s: %w", srcPath, err)
	}

	baseName := filepath.Base(relFile)
	flatHandler = strings.TrimSuffix(baseName, ".js") + "." + exportName

	f, err := os.CreateTemp("", "hels-deploy-*.zip")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create(baseName)
	if err != nil {
		return "", "", err
	}
	if _, err := w.Write(data); err != nil {
		return "", "", err
	}
	if err := zw.Close(); err != nil {
		return "", "", err
	}

	return f.Name(), flatHandler, nil
}
