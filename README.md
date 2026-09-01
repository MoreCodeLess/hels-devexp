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

### Entornos locales

```bash
hels env up dev       # levanta el entorno "dev" (o confirma que ya está corriendo)
hels env status dev   # muestra su estado
hels env list         # lista todos los entornos declarados en hels.yaml
hels env down dev     # lo baja

eval "$(hels env switch dev)"   # lo levanta, lo marca como activo, y exporta
                                 # AWS_ENDPOINT_URL/AWS_DEFAULT_REGION/credenciales
                                 # dummy para apuntar tu AWS SDK/CLI ahí
```

Cada entorno corre como un contenedor Docker de [floci](https://github.com/floci-io/floci)
(el motor de simulación de AWS), gestionado directamente vía `docker` — no hace
falta instalar el CLI de floci por separado. `up`/`switch` son idempotentes: si
el entorno ya está corriendo, no hacen nada. El entorno "activo" (el que usa
`switch`) se guarda en `.hels/state.json`, un archivo local **no versionado**
(ver `.gitignore`) — `hels.yaml` sigue siendo 100% declarativo y reproducible.

### Escanear servicios de Serverless Framework

```bash
hels scan ./mi-monorepo                          # texto legible (default)
hels scan ./mi-monorepo --format mermaid         # diagrama Mermaid
hels scan ./mi-monorepo --format json            # grafo completo en JSON
hels scan ./mi-monorepo --format hels-yaml > hels.yaml   # genera un hels.yaml de partida
```

Recorre la ruta buscando `serverless.yml`/`serverless.yaml` (salteando
`node_modules`, `.git`, `.serverless`, `vendor`), extrae funciones, eventos y
recursos de cada servicio, y detecta conexiones reales entre servicios por
tres vías, de más a menos confiable:

1. `${cf:otro-stack.Output}` — referencia explícita a un output de otro stack de CloudFormation.
2. `Fn::ImportValue` — contra lo que otro servicio exporta (`resources.Outputs.*.Export.Name`).
3. ARNs/nombres literales — un ARN que menciona una cola/tabla/tópico/bucket que otro servicio declaró (`QueueName`, `TopicName`, `TableName`, `BucketName`). Es la más heurística de las tres: dos servicios podrían coincidir en nombre por casualidad.

`--format hels-yaml` arma un `hels.yaml` de partida: un entorno `dev` con los
servicios de floci que hacen falta para simular todo lo que se detectó
(mirando los `events` de las funciones y los `Type` de los recursos de
CloudFormation). Es un punto de partida para revisar, no una verdad
absoluta — pero ya es cargable directo por `hels env`.

### Deploy real contra floci

```bash
hels deploy ./mi-monorepo            # crea todo contra el entorno "dev"
hels deploy ./mi-monorepo --env qa   # contra otro entorno declarado en hels.yaml
```

Escanea la ruta (igual que `hels scan`) y crea, contra floci, los recursos de
verdad: funciones Lambda (empaquetando el handler), colas SQS, tópicos SNS,
tablas DynamoDB y buckets S3. Requiere el **AWS CLI** instalado — `hels
deploy` habla con floci a través de él en vez de reimplementar la API de AWS.

**A propósito no pasa por CloudFormation** (que es lo que hace `serverless
deploy` por dentro): se probó ese camino contra floci y tiene un bug real —
la traducción de CloudFormation a Lambda falla con "Handler file ... not
found" incluso con el paquete armado bien. Yendo directo a la API de cada
servicio se evita ese problema por completo, y de paso no hace falta tener
Node.js/npm/`serverless-localstack` instalados.

Recursos que todavía no crea (API Gateway, Cognito, ElastiCache, ECS, ...) se
listan al final de la corrida como "salteados" — no se ignoran en silencio.
Para exponer una función por HTTP hoy, apuntá tu gateway (ej.
[KrakenD](https://www.krakend.io/)) directo al endpoint de invocación de
Lambda que ya expone floci sin necesitar autenticación:

```
POST http://localhost:4566/2015-03-31/functions/<nombre-de-la-función>/invocations
```

## Desarrollo

```bash
go build ./...
go vet ./...
go test ./...
```
