# hels-devexp

CLI de desarrollo local para equipos con Serverless Framework: simula tu nube AWS
en tu máquina (motor [floci](https://github.com/floci-io/floci)) a partir de un
único archivo `hels.yaml` reproducible y versionable en Git.

## Instalación

```bash
curl -fsSL https://raw.githubusercontent.com/MoreCodeLess/hels-devexp/main/install.sh | bash
```

Detecta tu SO (Linux/macOS) y arquitectura (amd64/arm64), descarga el binario del
último release, lo instala en `/usr/local/bin` (o `~/.local/bin` si el primero no
es escribible), y agrega ese directorio a tu `PATH` automáticamente (en
`~/.bashrc`/`~/.profile` o `~/.zshrc`/`~/.zprofile`, según tu shell) si hacía falta.
Si te lo agregó, abrí una terminal nueva (o una sesión SSH nueva) para que tome
efecto.

## Uso

```bash
hels init      # crea un hels.yaml de partida en el directorio actual
hels version   # muestra la versión instalada
hels upgrade   # descarga e instala la última versión, reemplazando el binario actual
```

`hels upgrade` no depende de `curl`/`bash` externos ni del script de instalación:
el propio binario consulta el último release de GitHub, descarga el `.tar.gz`
correspondiente a tu plataforma y se reemplaza a sí mismo (rename atómico, seguro
mientras sigue corriendo). Si ya estás en la última versión, no hace nada — usá
`--force` para reinstalar igual, o `--version vX.Y.Z` para fijar una versión
específica.

Ver el esquema completo de `hels.yaml` en [`hels.example.yaml`](./hels.example.yaml).

## Desarrollo

```bash
go build ./...
go vet ./...
go test ./...
```
