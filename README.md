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
Para exponer una función por HTTP hoy, apuntá tu gateway (KrakenD, nginx, el
que sea) directo al endpoint de invocación de Lambda que ya expone floci sin
necesitar autenticación:

```
POST http://localhost:4566/2015-03-31/functions/<nombre-de-la-función>/invocations
```

### Dashboard: todo en una vista, al estilo mprocs

`hels.yaml` puede declarar procesos locales — un gateway, un frontend, un
worker, lo que necesites correr en paralelo:

```yaml
processes:
  gateway:
    cmd: docker run --rm --name mi-gateway -p 8080:8080 -v ./gateway/config.json:/etc/krakend/krakend.json krakend run -c /etc/krakend/krakend.json
    dir: .
  frontend:
    cmd: npm run dev
    dir: ./frontend
```

**hels no sabe ni le importa qué hace cada comando** — no está "quemado"
ningún gateway en particular: declarás el que uses, donde lo tengas, y hels
lo corre tal cual. Después:

```bash
hels dashboard
```

Arranca **todos** los `processes.*` de una (igual que mprocs) y los muestra
arriba en la lista, con sus logs en vivo — y abajo, la infraestructura
Docker (floci y cualquier otro contenedor). Todo en una sola vista: front,
back, gateway e infra, sin acordarte de qué comando suelto correr en qué
terminal aparte.

- Click o `j`/`k` para elegir un servicio; `Tab` para pasarle el foco del teclado al panel de logs.
- `r` reinicia el proceso seleccionado (no aplica a contenedores de infra — esos se manejan con `hels env`).
- `q` sale, parando primero todos los procesos locales (no deja nada huérfano corriendo).

La lista de infra solo muestra contenedores que `hels` gestiona (floci y lo
que venga después) — no todo lo que esté corriendo en Docker en tu máquina.

**Menú de categorías**: arriba de la lista hay una fila de pestañas
clickeables (`[*] [>] [#] [λ] [Q] [T]` — todos, procesos, infra, Lambdas,
colas, tópicos) para saltar directo a una categoría sin scrollear entre
decenas de ítems; solo aparecen las que tienen algo. `←`/`→` (o `h`/`l`) las
cicla también desde el teclado. Con muchos servicios (un proyecto con
varias Lambdas, colas, etc.) la lista scrollea sola dentro de su categoría
— con el mouse (rueda sobre la lista) o con `j`/`k`, que llevan el cursor
más allá de lo visible.

**Funciones Lambda (`λ`)**: si el entorno declarado en `hels.yaml` está
arriba, el dashboard le pregunta a floci qué funciones tiene desplegadas y
las suma a la lista, con su estado (si floci tiene ahora mismo un contenedor
"caliente" corriendo para esa función, o "sin invocaciones recientes" si no).
Seleccionar una muestra los logs de ese contenedor en vivo, y si floci lo
recicla (por inactividad) o crea uno nuevo, el dashboard se reengancha solo
al que esté vivo.

**Corrección sobre una nota anterior**: en una vuelta previa se documentó
acá que floci no exponía la salida real de una función (console.log, stack
traces). Eso estaba mal — el chequeo original se hizo contra handlers que
literalmente nunca llamaban a `console.log`, así que no había nada que
mostrar, no importa el canal. Con un handler que sí loguea algo, tanto el
`docker logs` del contenedor de la invocación como CloudWatch Logs (vía
`FilterLogEvents` — `GetLogEvents` puntual devuelve vacío por una rareza de
floci, pero `FilterLogEvents` sí trae el contenido real) muestran la salida
real, confirmado de punta a punta. Así que esta vista sirve tanto para ver
si tu función está "caliente" como para ver de verdad qué imprimió.

**Colas SQS (`Q`) y tópicos SNS (`T`)**: mismo mecanismo — se listan las que
haya desplegadas contra el entorno activo. Una cola muestra en su estado
cuántos mensajes tiene visibles/en vuelo, y al seleccionarla el panel de
logs muestra hasta 10 mensajes reales (ID corto + body), refrescado en cada
ciclo. Es un **peek no destructivo** (`ReceiveMessage` con
`VisibilityTimeout=0`): no los saca de la cola ni se los esconde a otros
consumidores. Ojo: sí cuenta como una entrega más a efectos de
`ReceiveCount` — si tenés una redrive policy con `maxReceiveCount` muy
ajustado, tenerla seleccionada en el dashboard suma intentos igual que un
consumidor real.

Un tópico SNS no retiene histórico de publicaciones (así es SNS en AWS real
también), así que lo que se muestra al seleccionarlo es a quién reenvía lo
que se publique ahí (sus suscriptores — colas, funciones, lo que sea) en
vez de un log de eventos pasados.

## Desarrollo

```bash
go build ./...
go vet ./...
go test ./...
```
