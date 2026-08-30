# hels-devexp

CLI de desarrollo local para equipos con Serverless Framework: simula tu nube AWS
en tu máquina (motor [floci](https://github.com/floci-io/floci)) a partir de un
único archivo `hels.yaml` reproducible y versionable en Git.

## Instalación

```bash
curl -fsSL https://raw.githubusercontent.com/MoreCodeLess/hels-devexp/main/install.sh | bash
```

Detecta tu SO (Linux/macOS) y arquitectura (amd64/arm64), descarga el binario del
último release y lo instala en `/usr/local/bin` (o `~/.local/bin` si el primero no
es escribible).

## Uso

```bash
hels init      # crea un hels.yaml de partida en el directorio actual
hels version   # muestra la versión instalada
```

Ver el esquema completo de `hels.yaml` en [`hels.example.yaml`](./hels.example.yaml).

## Desarrollo

```bash
go build ./...
go vet ./...
go test ./...
```
