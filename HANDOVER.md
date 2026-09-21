# 🧅 Allium — Documento de Traspaso

> Estado del proyecto al 8 de agosto de 2026. Escrito para que alguien que nunca ha visto
> este repo pueda entenderlo, compilarlo, y continuarlo sin preguntar nada.

## Índice

1. [Qué es Allium](#1-qué-es-allium)
2. [Stack y librerías: qué usa y por qué](#2-stack-y-librerías)
3. [Cómo compilar y correrlo](#3-cómo-compilar-y-correrlo)
4. [Arquitectura: cómo se conecta todo](#4-arquitectura)
5. [Modelo de datos](#5-modelo-de-datos)
6. [Recorrido archivo por archivo](#6-recorrido-archivo-por-archivo)
7. [Contrato de la API](#7-contrato-de-la-api)
8. [Los cuatro flujos importantes](#8-los-cuatro-flujos-importantes)
9. [Estado real: qué funciona y qué no](#9-estado-real)
10. [Bugs conocidos](#10-bugs-conocidos)
11. [Por dónde seguir](#11-por-dónde-seguir)
12. [Cómo probar el proyecto](#12-cómo-probar-el-proyecto)
13. [Go básico: lo que necesitas saber para este repo](#13-go-básico)
14. [Glosario de decisiones](#14-glosario-de-decisiones)

---

## 1. Qué es Allium

Allium convierte cualquier computadora (una Raspberry Pi, una PC vieja) en un **servidor privado
de fotos**. Tú importas tu export de Google Takeout, Allium las indexa localmente en SQLite,
genera miniaturas, y sirve una galería web. Lo distintivo: el acceso remoto es **exclusivamente
por Tor**, como un Hidden Service `.onion`. No hay que abrir puertos, ni IP pública, ni NAT
traversal, ni una cuenta en ningún lado. Los archivos nunca salen del hardware del usuario.

Se distribuye en **dos binarios que comparten el mismo núcleo**:

- **`allium-node`** — CLI headless. Es el que corre 24/7 en un servidor sin pantalla.
- **`allium-desktop`** — ventana nativa (Wails) para usuarios finales.

Ambos construyen un `core.App` y llaman a sus métodos; toda la lógica de negocio vive ahí.

**Nombre del módulo Go:** `allium-server` (los imports son `allium-server/internal/...`).
**Rama principal:** `main`. Un solo commit hasta ahora: `406043b desktop version`.

---

## 2. Stack y librerías

### Backend (Go 1.26)

| Librería | Para qué se usa aquí | Notas de peso |
|---|---|---|
| `github.com/uptrace/bun` + `dialect/sqlitedialect` | ORM. Se usa el query builder (`NewSelect`, `NewInsert`, `ScanAndCount`) y la automigración `NewCreateTable().IfNotExists()` | Los modelos llevan tags `bun:"..."`. La relación m2m Album↔Photo requiere `db.RegisterModel()` **antes** de cualquier query — se hace en `db.InitDB`. |
| `modernc.org/sqlite` | Driver SQLite **puro Go, sin CGO** | Elegido deliberadamente: permite compilación cruzada trivial a `linux/arm64` para Raspberry Pi. Se abre con `sql.Open("sqlite", path)` (ojo: `"sqlite"`, no `"sqlite3"`). |
| `github.com/davidbyttow/govips/v2` | Binding de **libvips** para redimensionar y exportar a WebP | **Esta sí usa CGO.** Requiere `libvips-dev` instalado en el sistema (`apt-get install libvips-dev` / `brew install vips`). Es la única razón por la que el build no es 100% portable — contradice parcialmente la elección de `modernc.org/sqlite`. Hay que llamar `vips.Startup(nil)` una vez y `vips.Shutdown()` al salir. |
| `github.com/buckket/go-blurhash` | Genera el string BlurHash para el placeholder borroso mientras carga la miniatura | Necesita un `image.Image` de Go, no bytes; por eso se decodifica el WebP recién generado. |
| `golang.org/x/image/webp` | Decodificar el WebP en memoria para dárselo a blurhash | Sólo existe en el proyecto para servir de puente entre govips y go-blurhash. |
| `github.com/rwcarlsen/goexif` | Leer EXIF de las fotos (fecha, GPS, autor, descripción) | No soporta HEIC — el código lo detecta y devuelve metadatos vacíos con un aviso. |
| `github.com/cretz/bine` | Arranca y controla un proceso Tor embebido, crea el Hidden Service | `tor.Start` → `EnableNetwork` → `t.Listen(&tor.ListenConf{RemotePorts: []int{80}, LocalPort: apiPort, Key: key})`. La `Key` ed25519 se persiste en `<dataDir>/onion_key` para conservar la misma dirección `.onion` entre reinicios. |
| `github.com/spf13/cobra` | Estructura de subcomandos de la CLI | Un archivo por grupo de comandos en `cmd/allium-node/cmd/`. |
| `golang.org/x/crypto/bcrypt` | Hash de contraseñas — en **dos lugares distintos**: perfiles del SO y usuarios de la API web | Ver §12, son dos sistemas de auth separados a propósito. |
| `golang.org/x/term` | Leer la contraseña de la terminal sin eco | Usado en `profile create` / `profile login`. |
| `github.com/wailsapp/wails/v2` | Ventana nativa del binario desktop | Usa el WebView del sistema operativo, no empaqueta Chromium. |

Librerías estándar que hacen trabajo pesado: `net/http` (con el **router de Go 1.22+**:
patrones tipo `"GET /api/photos/{id}"` y `r.PathValue("id")` — no hay ningún router de terceros),
`archive/zip`, `crypto/sha256`, `os/exec` + `syscall` (daemonización), `embed`.

> ⚠️ `goexif` y `wails` aparecen en el bloque `// indirect` de `go.mod` aunque se importan
> directamente. Un `go mod tidy` los moverá al bloque principal. Es cosmético.

### Frontend (`ui/`)

| Librería | Para qué |
|---|---|
| React 19 + `react-dom` | Toda la UI. **Sin router** — la navegación es un `useState<View>` en `App.tsx`. |
| Vite 8 + `@vitejs/plugin-react` | Bundler. Dev server en `:5173` con proxy `/api` (ver nota de puerto en §10). |
| TypeScript 6 | `types.ts` refleja a mano los structs de Go. No hay generación automática. |
| ESLint 10 | Linting. |

**No hay Tailwind, ni librería de componentes, ni state manager.** Todo el estilo son ~640 líneas
de CSS a mano en `ui/src/index.css` con custom properties (`--color-bg`, `--radius-md`, etc.) en un
tema oscuro estilo Google Photos. Fuentes: se importan desde Google Fonts por URL — un detalle a
revisar si te importa que la galería no haga peticiones externas cuando se sirve por Tor.

El build de Vite sale a **`../internal/ui/dist`** (no a `ui/dist`), y ahí lo recoge
`//go:embed all:dist` para quedar dentro del binario Go. Por eso `make build-node` depende de
`build-ui`, y por eso `internal/ui/dist/` está en `.gitignore` (es artefacto).

---

## 3. Cómo compilar y correrlo

**Prerrequisitos:** Go 1.26+, Node.js, `libvips-dev`. Para el desktop además la CLI de Wails
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```bash
make build-ui        # Vite → internal/ui/dist/
make build-node      # (corre build-ui primero) → build/allium-node
make build-node-arm  # cross-compile linux/arm64 para Raspberry Pi
make build-desktop   # requiere wails en el PATH
make test            # go test -v -race ./...   (ver advertencia en §10)
make dev-ui          # Vite dev server :5173
make dev-api         # live-reload con air (.air.toml)
make help            # ayuda autogenerada de los comentarios ##
```

Uso típico de punta a punta:

```bash
./build/allium-node profile create juan     # pide contraseña dos veces
./build/allium-node profile login juan      # activa el perfil para esta sesión
./build/allium-node add-photos ~/Takeout/   # importa (el servidor debe estar apagado)
./build/allium-node start --tor             # arranca en background
./build/allium-node ps                      # tabla de instancias corriendo
./build/allium-node stop
./build/allium-node uninstall               # borra ~/.allium por completo
```

Puerto por defecto **41110**, escuchando sólo en `127.0.0.1`. Se puede cambiar con `--port` o con
la variable de entorno `ALLIUM_PORT`.

### Dónde vive todo en disco

```
~/.allium/
├── session.json                 ← qué perfil está activo (global)
└── <perfil>/
    ├── profile.json             ← nombre, fecha, hash bcrypt de la contraseña del perfil
    ├── server.pid               ← "PID\nperfil\ntimestamp"
    ├── server.log               ← stdout+stderr del daemon
    ├── run/
    └── data/
        ├── allium.db            ← SQLite (WAL)
        ├── photos/<hash>.<ext>  ← originales, nombrados por su SHA256
        └── thumbs/<hash>.webp   ← miniaturas
```

---

## 4. Arquitectura

```
                        ┌─────────────────────────────────────────────┐
                        │  cmd/allium-node/main.go                    │
                        │  ¿os.Args contiene --daemon-mode?           │
                        └───────────┬──────────────────┬──────────────┘
                            NO      │                  │  SÍ
                                    ▼                  ▼
                   ┌────────────────────────┐   ┌──────────────────┐
                   │ cmd/.../cmd (Cobra)    │   │ runDaemon()      │
                   │  root · profile        │   │  perfil activo   │
                   │  server(start/stop/ps) │   │  arma models.    │
                   │  photos(add-photos) ●  │   │  Config          │
                   │  uninstall             │   └────────┬─────────┘
                   └───┬────────────┬───────┘            │
                       │            │                    ▼
      internal/profile ◄            ▼        ┌───────────────────────────┐
      (~/.allium/<p>/) │   internal/daemon   │ internal/core.App         │
       · Create/Login  │   fork + PID file   │  Start(ctx):              │
       · rutas del FS  │   SIGTERM/SIGKILL   │   1. db.InitDB      ✓     │
                       │                     │   2. image.NewProcessor ✓ │
                       │                     │   3. api.NewServer  ⚠     │
                       │                     │   4. tor.Controller ✓     │
                       │                     └──────┬────────────────────┘
   ┌───────────────────┴──────────┐                 │  ⚠ NO le pasa
   │ ● la ingesta REAL vive aquí  │                 │    processor ni torCtrl
   │   (duplicada; internal/      │                 ▼
   │    downloads quedó en stub)  │      ┌────────────────────────────┐
   │   retrieveGroups → unzip     │      │ internal/api.Server        │
   │   pickBestImage              │      │  http.ServeMux (Go 1.22)   │
   │   processGroup:              │      │  auth() middleware Bearer  │
   │    SHA256→dedup→copy→thumb   │      ├────────────────────────────┤
   └───┬──────────────┬───────────┘      │ POST /api/auth/register ✓  │
       │              │                  │ POST /api/auth/login    ✓  │
       ▼              ▼                  │ GET  /api/photos        ✓  │
 internal/image  internal/storage        │ GET  /api/photos/{id}   ✓  │
  · ComputeSHA256   · PhotoRepository    │ GET  /api/photos/{id}/thumb│
  · Thumb+Blurhash  · AlbumRepository    │ GET  /api/albums        ~  │
  · ExtractEXIF          │               │ GET  /api/status      ✗💥  │
  · ProcessMetadataJSON  ▼               │ POST /api/ingest      ✗    │
  (govips + goexif)  internal/db         │ GET  /  → embed dist ✓     │
                     (Bun + SQLite WAL)  └──────────┬─────────────────┘
                          │                         │
                          ▼                         │  ⚠ JSON en PascalCase
                  internal/models                   │
                   Photo·Album·AlbumPhoto           ▼
                   User·Session·Config     ┌────────────────────────┐
                   (⚠ sin tags json)       │ ui/ (React + Vite)     │
                                           │  api.ts → fetch /api   │
   internal/tor.Controller ✓               │  App.tsx  · AuthPage   │
    bine → Start → Listen(:80→:41110)      │  PhotoCard · Lightbox  │
    persiste onion_key                     │  StatusBar             │
                                           │  types.ts (⚠ camelCase)│
   internal/downloads ✗ (stub)             └────────────────────────┘
   internal/uploads   ✗ (reservado a futuro)          ▲
                                                      │ go:embed all:dist
                                            internal/ui/embed.go
```

Leyenda: `✓` funciona · `~` parcial · `⚠` cableado incompleto · `✗` no implementado · `💥` panic

**La regla de capas que respeta el proyecto:** sólo `internal/storage` (y `internal/api/auth.go`)
tocan `*bun.DB`. Los handlers hablan con repositorios. `core.App` es el único que conoce a todos
los subsistemas.

---

## 5. Modelo de datos

Cinco tablas, automigradas al arrancar en `db.runMigrations`:

```
photos            albums             album_photos          users        sessions
──────            ──────             ────────────          ─────        ────────
id       PK       id       PK        album_id  PK,FK       id      PK   token    PK
hash     UNIQUE   name                photo_id  PK,FK      username     user_id
title             description                              UNIQUE       username
description       cover_photo_id                           password_    created_at
artist            created_at                               hash
captured_at IDX   updated_at                               created_at
latitude
longitude
file_path
thumb_path
blurhash
created_at
```

- **`photos.hash`** es el SHA256 del archivo original y es la **clave de deduplicación**. El
  insert usa `ON CONFLICT (hash) DO NOTHING`, y además el archivo en disco se nombra `<hash>.<ext>`.
- **`captured_at`** es un Unix timestamp `int64` (no `time.Time`) — decisión de velocidad para
  ordenar la galería sin parsear fechas.
- **`album_photos`** es la tabla pivote m2m. Existe y está registrada, pero **nada la escribe
  todavía**: la ingesta no crea álbumes.
- **`sessions`** guarda los tokens en BD (en vez de JWT) precisamente para poder invalidarlos.
  Hoy no hay endpoint de logout ni expiración, así que los tokens son eternos.

---

## 6. Recorrido archivo por archivo

### `cmd/allium-node/main.go` (137 líneas)
El binario decide en la primera línea de `main()` qué es: si `daemon.IsDaemonMode()` (busca
`--daemon-mode` en `os.Args`) corre `runDaemon()`; si no, entrega el control a Cobra.
- `runDaemon()` — resuelve el perfil (flag `--profile` o sesión activa), arma el `models.Config`
  con las rutas de ese perfil, crea el `core.App`, y bloquea en `<-ctx.Done()` con
  `signal.NotifyContext(SIGINT, SIGTERM)`.
- `parseDaemonArgs()` — parseo manual de `--profile/--port/--tor`. Deliberadamente no usa Cobra,
  para que el proceso hijo arranque rápido.

### `cmd/allium-node/cmd/` — la CLI

**`root.go`** — `rootCmd` de Cobra y `Execute()`. Todos los subcomandos se registran en su `init()`.

**`profile.go`** — `profile create|login|logout|list`. Lee contraseñas con `term.ReadPassword`
(sin eco), delega todo a `profile.Manager`. `formatAge()` formatea "hace 3h". **Completo.**

**`server.go`** — `start`, `stop`, `ps`. `runStart` traduce las flags a argumentos extra y llama a
`daemon.Start`; `runPS` recorre todos los perfiles y pinta una tabla con `text/tabwriter` marcando
cuál está corriendo y cuál es la sesión activa. `formatUptime()` da "2d4h". **Completo.**

**`photos.go` (371 líneas) — el archivo más importante y el más problemático.**
Aquí vive **toda la ingesta real**, aunque conceptualmente debería estar en `internal/downloads`.
- `runAddPhotos` — resuelve el perfil y **se rehúsa a correr si el servidor está encendido**
  (para no pelear por el lock de SQLite); simplemente imprime un aviso y sale.
- `runIngestDirect` — abre BD y procesador, escanea, y monta un **worker pool de 8 goroutines**
  con canales `jobs`/`results` y un `sync.WaitGroup`.
- `retrieveGroups` — `filepath.WalkDir` recursivo que agrupa archivos por nombre base en un
  `map[string]*FileGroup{ImagePaths, JSONPath, BaseName}`. Si encuentra `.zip` lo descomprime y
  recursa (con un contador `deepness` de 5 para no colgarse en zips anidados).
- `pickBestImage` — cuando el mismo nombre tiene varios formatos, prioriza **HEIC > PNG > JPG > MP4**.
- `processGroup` — el pipeline por foto: `ComputeSHA256` → `GetByHash` (dedup) → copiar/mover a
  `photos/<hash><ext>` → metadatos (JSON de Takeout si existe, si no EXIF) → `GenerateThumbnail
  AndBlurHash` → `repo.Save`.
- `unzipTo`, `copyFile` — helpers.
- Flags: `--dry-run`, `--move`, `-r/--recursive` (**esta última se declara y nunca se lee**).

**`uninstall.go`** — resumen de lo que va a borrar, confirmación interactiva (acepta si/sí/yes/y)
o `--force`, detiene todos los daemons y llama `mgr.DeleteAll()`. **Completo.**

### `cmd/allium-desktop/main.go` (43 líneas)
Crea el `core.App` con `DefaultConfig()` y llama `wails.Run` con `OnStartup: app.Startup`,
`OnShutdown: app.Shutdown`, sirviendo `ui.Dist` desde el asset server de Wails y haciendo `Bind`
del app. **Ver §10 — en este modo el frontend no encuentra la API.**

### `internal/core/app.go` (138 líneas) — el cerebro
- `New(cfg)` — hoy sólo guarda config y logger; las validaciones siguen como TODO.
- `Start(ctx)` — arranca en orden: **[1]** `db.InitDB` → **[2]** `image.NewProcessor` →
  **[3]** `api.NewServer` + `s.Start()` en una goroutine → **[4]** `tor.NewController` + `Start`
  si `cfg.TorEnabled`. El comentario promete rollback si algo falla; no está implementado.
- `Stop()` — apaga Tor, procesador y BD. **Sin nil-checks** (ver §10).
- `Startup(ctx)` / `Shutdown(ctx)` — hooks que Wails llama; envuelven `Start`/`Stop`.
- `DB()` — expone la conexión; sólo para inicialización.

### `internal/models/` — structs compartidos
`photo.go`, `album.go` (+ `AlbumPhoto`), `user.go` (+ `Session`), `config.go`.
`DefaultConfig()` devuelve rutas bajo `~/.allium`, puerto 41110, Tor apagado, thumbs de 400px de
ancho (alto proporcional), 4 workers. **Ningún struct tiene tags `json:` — ver §10, bug #1.**

### `internal/db/database.go` (87 líneas)
- `InitDB(dbPath)` — `MkdirAll` del directorio, `sql.Open("sqlite", ...)`, `PRAGMA journal_mode=WAL`,
  `PRAGMA foreign_keys=ON`, envuelve con Bun, `RegisterModel(AlbumPhoto)` y migra.
- `runMigrations(db)` — `CREATE TABLE IF NOT EXISTS` para los 5 modelos. **Si añades un modelo
  nuevo, tienes que agregarlo a este slice.** No hay versionado de migraciones ni manejo de
  cambios de esquema: si alteras un struct existente, la tabla vieja se queda como está.

### `internal/storage/` — acceso a datos
`photo_repo.go`: `NewPhotoRepository`, `Save` (con `ON CONFLICT (hash) DO NOTHING`), `GetByID`,
`GetByHash` (devuelve `nil, nil` si no existe — así se detecta el duplicado), `ListPaginated`
(`ScanAndCount` ordenando por `captured_at DESC`), `Delete` (**no borra el archivo físico**, es
responsabilidad de quien llama). `album_repo.go`: los mismos cuatro, ordenando por `updated_at`.
Falta un `Count()` y métodos para poblar `album_photos`.

### `internal/api/`
**`server.go`** — el `Server` tiene campos `processor` y `torCtrl` que **`NewServer` nunca recibe**.
`registerRoutes()` declara las rutas; `auth()` es el middleware que lee `Authorization: Bearer <t>`
y busca el token en la tabla `sessions`. `Start()` es un `http.ListenAndServe("127.0.0.1:port")`
sin timeouts ni shutdown grácil.

**`auth.go`** — `handleRegister` (valida usuario ≥3 y contraseña ≥8, hashea con bcrypt, 409 si ya
existe), `handleLogin` (compara hash, genera token de 32 bytes hex con `crypto/rand`, lo inserta en
`sessions`), `generateToken`. **Completo.**

**`handlers.go`** — `handleListPhotos` (paginación con tope de 200) ✓ · `handleGetPhoto` ✓ ·
`handleServeThumb` (usa `http.ServeFile`, así hereda Range requests y ETag gratis) ✓ ·
`handleListAlbums` (devuelve `{albums,total}`, sin cover ni fotos) ~ · `handleIngest` (escrito para
emitir **SSE** desde el canal `Progress` del Ingester, pero el Ingester no existe) ✗ ·
`handleStatus` (**panickea**, ver §10) ✗ · `handleFrontend` (sirve `internal/ui/dist` embebido; si
la ruta no existe como archivo, reescribe a `/` para que funcione el routing del cliente) ✓ ·
`writeJSON` (helper).

### `internal/image/processor.go` (264 líneas)
- `NewProcessor` — `vips.Startup(nil)` + `MkdirAll`. **Se llama una sola vez por proceso.**
- `Shutdown` — `vips.Shutdown()`.
- `GenerateThumbnailAndBlurHash(srcPath, hash)` — carga con vips, calcula la escala como
  `thumbWidth/anchoOriginal`, redimensiona con **Lanczos3**, exporta a WebP, decodifica ese WebP
  para calcular el BlurHash (componentes 4×3), escribe a `thumbs/<hash>.webp`, y devuelve
  `ProcessResult{ThumbPath, Blurhash, Width, Height}` con las dimensiones **originales**.
- `ComputeSHA256(filePath)` — streaming con `io.Copy(hasher, file)`, no carga el archivo en RAM.
- `ExtractEXIF(srcPath)` — fecha, lat/lon, descripción, autor. HEIC no soportado (avisa y devuelve
  vacío). Ante error no crítico continúa.
- `ProcessMetadataJSON(srcPath)` — parsea el JSON de Google Takeout (`title`, `description`,
  `photoTakenTime.timestamp` que Google manda como **string**, `geoData.latitude/longitude`).
- Structs: `ProcessResult`, `Processor`, `EXIFData`, `GoogleMetadata`.

### `internal/tor/controller.go` (139 líneas)
- `NewController(dataDir, apiPort)` — sólo crea el directorio; no arranca nada.
- `Start(ctx)` — `tor.Start` → `EnableNetwork` (aquí es donde tarda **30–60s la primera vez**,
  bootstrapeando) → lee o genera la key ed25519 en `<dataDir>/onion_key` (0600) →
  `t.Listen` mapeando el puerto 80 de la `.onion` al puerto local de la API.
- `Stop()` — cierra el onion y el proceso Tor, con nil-checks.
- `OnionAddress()` — `http://<id>.onion`, vacío antes de `Start`.

**Reutilizar la key es lo que hace que tu dirección `.onion` no cambie entre reinicios.** Si borras
`onion_key`, todos tus enlaces guardados dejan de funcionar.

### `internal/daemon/daemon.go` (233 líneas)
Daemonización **sin systemd ni launchd**: el binario se re-ejecuta a sí mismo con `--daemon-mode`.
- `Start(profileName, pidFile, extraArgs)` — `os.Executable()` → `exec.Command` con
  `SysProcAttr{Setsid: true}` (nueva sesión ⇒ no recibe SIGHUP al cerrar la terminal), stdout y
  stderr a `server.log`, escribe el PID file, espera 300 ms y verifica con `Signal(0)` que no
  murió al instante, y `Process.Release()`.
- `Stop(pidFile)` — SIGTERM, poll cada 200 ms hasta 10 s, y SIGKILL si no obedece.
- `ReadPIDFile` — parsea el formato de 3 líneas y comprueba si el proceso vive con `kill -0`.
- `IsRunning`, `IsDaemonMode`, `writePID`.

### `internal/profile/profile.go` (321 líneas)
Multi-usuario a nivel de sistema operativo: cada perfil es un directorio aislado con su propia BD.
`Manager` con `New()` (usa `$HOME`) y `NewWithBase()` (**pensado para tests**). Métodos de rutas
(`ProfileDir`, `DataDir`, `DBPath`, `ThumbsDir`, `PhotosDir`, `PIDFile`, `RunDir`), operaciones
(`Create`, `Login`, `Logout`, `ActiveProfile`, `ActiveSession`, `Exists`, `List`, `DeleteProfile`,
`DeleteAll`, `BaseDir`) y helpers (`validateName` — sólo `[a-zA-Z0-9_-]` y rechaza los nombres
reservados `session`/`run`/`tmp`; `writeJSON`/`readJSON` con permisos 0600). Errores exportados:
`ErrNoActiveProfile`, `ErrProfileExists`, `ErrProfileNotFound`, `ErrInvalidPassword`. **Completo.**

### `internal/downloads/ingester.go` (116 líneas) — ❌ STUB
Los structs están bien diseñados (`Ingester`, `IngestConfig`, `ProgressEvent`) y `NewIngester`
funciona, pero `Start`, `parseGoogleMetadata` y `AddSingleFile` devuelven `not implemented`.
El comentario de cabecera describe exactamente el pipeline que ya está escrito en `cmd/.../photos.go`.

### `internal/uploads/exporter.go` (44 líneas) — ❌ RESERVADO
Exportar de vuelta a Google Photos vía OAuth2. Nada implementado, y no bloquea nada.

### `internal/ui/embed.go` (5 líneas)
`//go:embed all:dist` → `var Dist embed.FS`. El `all:` es necesario para incluir archivos que
empiezan con `_` o `.`.

### `ui/src/`
- **`main.tsx`** — monta `<App/>` en `#root` con `StrictMode`.
- **`App.tsx` (188)** — guarda el token en `localStorage['allium_token']`; si no hay, renderiza
  `<AuthPage/>`. Con token, pide 200 fotos, filtra por búsqueda en cliente (título y descripción)
  y las agrupa por fecha con `groupByDate` (locale `es-MX`). Navbar + sidebar + grid + lightbox +
  status bar.
- **`api.ts` (71)** — `login`, `register`, `fetchPhotos`, `fetchPhoto`, `thumbUrl`, `fetchAlbums`,
  `fetchStatus`, `startIngest` (stub). `authHeaders()` lee el token de localStorage.
- **`types.ts` (60)** — espejo manual de los structs de Go. `Photo`, `Album`, `ServerStatus`,
  `PhotosResponse`, `AuthResponse`, `IngestProgressEvent`.
- **`components/AuthPage.tsx` (186)** — login/registro con tabs, validación en cliente, estados de
  loading y error. La pantalla más pulida del proyecto.
- **`components/PhotoCard.tsx` (35)** — miniatura con `loading="lazy"` y un placeholder hasta que
  carga. Accesible (`role="button"`, Enter).
- **`components/Lightbox.tsx` (61)** — modal con cierre por Escape y por click en el backdrop.
- **`components/StatusBar.tsx` (63)** — semáforo del estado de Tor y `.onion` copiable al portapapeles.

---

## 7. Contrato de la API

Todo bajo `/api`. Salvo los dos de auth, **todos requieren `Authorization: Bearer <token>`**.

| Método y ruta | Cuerpo / Query | Respuesta | Estado |
|---|---|---|---|
| `POST /api/auth/register` | `{username, password}` | `201` sin cuerpo · `400` validación · `409` duplicado | ✓ |
| `POST /api/auth/login` | `{username, password}` | `{token, username}` · `401` | ✓ |
| `GET /api/photos` | `?limit=50&offset=0` (tope 200) | `{photos:[...], total:N}` | ✓ |
| `GET /api/photos/{id}` | — | `{photo:{...}}` · `404` | ✓ |
| `GET /api/photos/{id}/thumb` | — | `image/webp` | ✓ |
| `GET /api/albums` | `?limit&offset` | `{albums:[...], total:N}` | ~ sin cover ni fotos |
| `GET /api/status` | — | `{status, onion}` | 💥 panic |
| `POST /api/ingest` | `{source_path, dry_run}` | `text/event-stream` (SSE) | ✗ siempre 400 |
| `GET /*` | — | el SPA embebido | ✓ |

**No existen** (y la UI los necesitaría): servir la foto **original** a tamaño completo, logout,
borrar foto, crear álbum.

---

## 8. Los cuatro flujos importantes

**Arranque del servidor.** `allium-node start` → `profile.ActiveProfile()` → `daemon.Start()`
re-ejecuta el binario con `--daemon-mode --profile X` y `Setsid` → el hijo entra por `runDaemon()`
→ `core.App.Start()` levanta BD, procesador, HTTP y (opcionalmente) Tor → el padre escribe el PID
y termina. `stop` lee el PID y manda SIGTERM.

**Ingesta de fotos.** `add-photos <ruta>` → `retrieveGroups` camina el árbol agrupando por nombre
base y descomprimiendo zips → 8 workers consumen los grupos → por cada uno: SHA256, consulta de
duplicado por hash, copia a `photos/<hash><ext>`, metadatos (JSON de Takeout con prioridad sobre
EXIF), miniatura WebP + BlurHash, insert en SQLite.

**Autenticación web.** Registro → bcrypt → tabla `users`. Login → compara → token aleatorio de 32
bytes → fila en `sessions` → el cliente lo guarda en `localStorage` y lo manda en cada petición →
el middleware `auth()` lo busca en BD. **Nunca expira** y no hay forma de invalidarlo.

**Publicación en Tor.** `--tor` → `bine` arranca un Tor embebido → bootstrap de la red (30–60 s la
primera vez) → lee o crea la key ed25519 persistida → registra el Hidden Service mapeando el
puerto 80 de la `.onion` al 41110 local → `OnionAddress()` queda disponible.

---

## 9. Estado real

**Compila limpio.** `go build ./cmd/allium-node` → OK. `go vet ./...` → 0 hallazgos.

| Subsistema | Estado |
|---|---|
| Perfiles multiusuario | ✅ Completo |
| Daemon (start/stop/ps) | ✅ Completo |
| Base de datos y repositorios | ✅ Completo (falta `Count` y m2m) |
| Procesamiento de imágenes | ✅ Funciona (sin auto-rotación EXIF) |
| Tor Hidden Service | ✅ Funciona, con key persistente |
| Ingesta por CLI | 🟡 Funciona, pero no empareja los JSON de Takeout |
| API REST | 🟡 5 de 8 endpoints correctos |
| Frontend React | 🟡 Bien construido, pero no recibe datos usables |
| Auth web | 🟡 Login/registro sí; sin logout ni expiración |
| Álbumes | 🔴 Tablas y repo listos, nada los llena |
| Ingesta por web (SSE) | 🔴 No implementada |
| Binario desktop | 🔴 Abre la ventana, pero la API no responde |
| Exportar a Google | 🔴 Reservado a futuro |
| Tests | 🔴 Existen, pero sin una sola aserción |

**En una frase:** la CLI importa fotos correctamente a SQLite, pero **la galería web no muestra
ni una sola imagen** por los tres cortes de la sección siguiente.

---

## 10. Bugs conocidos

Ordenados por impacto. Los tres primeros son los que rompen el producto.

**1. 🔴 Los nombres de campo del JSON no coinciden entre Go y TypeScript.**
`models.Photo` no tiene tags `json:`, así que `encoding/json` usa los nombres de campo de Go:
la API responde `{"ID":1,"CapturedAt":...}`. Pero `ui/src/types.ts` espera `{id, capturedAt}`.
Resultado: `photo.id` es `undefined`, `thumbUrl()` pide `/api/photos/undefined/thumb`, y **todas
las miniaturas fallan**. Arreglo: añadir tags camelCase a todos los modelos. De paso, `types.ts`
declara un `altitude` que no existe en Go y omite `artist`, `width` y `height`.

**2. 🔴 `GET /api/status` panickea.** `api.Server.torCtrl` nunca se asigna (`NewServer` no lo
recibe), y `handleStatus` llama `s.torCtrl.OnionAddress()` sobre un puntero nil. Con una sesión
válida la petición muere. Mismo origen: `core.App.Start()` crea el `Processor` y el `Controller`
pero no se los pasa al `Server`. Además el handler no devuelve `totalPhotos`, `version` ni
`torEnabled`, que es justo lo que la `StatusBar` espera.

**3. 🔴 No hay forma de importar fotos con el servidor encendido.** `POST /api/ingest` llama a
`downloads.Ingester.Start`, que es un stub; y `add-photos` se rehúsa a correr si el daemon está
activo (por el lock de SQLite). Cualquiera de los dos caminos hay que abrirlo.

**4. 🟠 `App.Stop()` panickea cuando Tor está apagado.** Llama `a.torCtrl.Stop()` sin comprobar
nil; con `TorEnabled=false` el controlador nunca se creó. Igual con `a.processor.Shutdown()` si
`Start` falló antes del paso [2]. Además cierra la BD dos veces (`a.db.Close()` y luego otra vez
dentro del `if a.db != nil`).

**5. 🟠 Los JSON de Google Takeout nunca se emparejan con su foto.** `retrieveGroups` agrupa con
`TrimSuffix(base, ext)`: para `IMG_1234.jpg` da `IMG_1234`, pero para
`IMG_1234.jpg.supplemental-metadata.json` da `IMG_1234.jpg.supplemental-metadata`. No coinciden,
así que **siempre cae al fallback de EXIF** y se pierden título, descripción y GPS de Takeout.
Hay un archivo de ejemplo real en `tests/` para reproducirlo.

**6. 🟠 El binario desktop no puede hablar con su propia API.** El frontend hace `fetch('/api/...')`,
que en Wails resuelve contra el asset server de la ventana, no contra el HTTP de Go en
`127.0.0.1:41110`. Hay que decidir entre apuntar la URL base al puerto real, o exponer métodos de
`core.App` por el `Bind` de Wails y llamarlos desde JS.

**7. 🟠 `unzipTo` no valida path traversal (zip-slip).** Un ZIP malicioso con entradas
`../../..//etc/algo` escribiría fuera del destino. Además descomprime **dentro del directorio
fuente del usuario**, ensuciando sus carpetas; debería usar `os.MkdirTemp`.

**8. 🟡 El Lightbox muestra la miniatura, no el original.** Usa `/api/photos/{id}/thumb`, o sea
400 px. No existe endpoint que sirva `photo.FilePath`.

**9. 🟡 `fetchStatus()` se llama antes del login.** Está en un `useEffect(…, [])` sin token, así
que siempre recibe 401 y cae al fallback. Debería moverse al efecto que depende de `token`.

**10. 🟡 El error de `blurhash.Encode` se ignora.** En `GenerateThumbnailAndBlurHash` el `err` se
reasigna inmediatamente después sin comprobarse; si falla, el BlurHash queda vacío en silencio.

**11. 🟡 Los tests no prueban nada.** Ambos archivos imprimen con `fmt.Println` sin aserciones,
usan **rutas absolutas al home de este equipo**, y el de Tor abre una conexión real a la red.
`make test` corre con `-race`, así que en otra máquina fallará o colgará.

**12. 🟡 Nits varios.** El proxy de dev de Vite apunta a `localhost:8080` pero el servidor escucha
en **41110** — `make dev-ui` no conecta con `make dev-api`. Los mensajes de error de
`PhotoRepository.Delete` dicen "album". Quedan `fmt.Println` de depuración en
`tor/controller.go` ("Ya existe" / "No existia") y en `image/processor.go`. La flag `--recursive`
se declara y nunca se usa. El campo `Processor.copyPath` está muerto. `flagDetach` en `server.go`
también. Los ítems "Álbumes" y "Favoritos" del sidebar cambian de estado pero renderizan lo mismo.
`http.ListenAndServe` sin timeouts. Sin límite de tamaño en el cuerpo de las peticiones.
`index.css` carga fuentes desde Google Fonts (petición externa desde una galería que presume de
privacidad).

---

## 11. Por dónde seguir

**Etapa 0 — Que la galería muestre fotos (bugs 1, 2, 4, 9).** Es medio día de trabajo y desbloquea
todo lo demás. Tags `json:` en los modelos, alinear `types.ts`, extender `NewServer` para recibir
`processor` y `torCtrl` (ojo: Tor se crea *después* del server en `App.Start`, así que o se
reordena o se añade un setter), nil-checks en `App.Stop`, y completar `handleStatus` con el conteo
de fotos y la versión que ya inyecta el `Makefile` por `ldflags`.

**Etapa 1 — Unificar la ingesta (bugs 3, 5, 7).** Mover `retrieveGroups`, `pickBestImage`,
`processGroup`, `unzipTo` y `copyFile` de `cmd/allium-node/cmd/photos.go` a
`internal/downloads/ingester.go`, convirtiéndolas en métodos del `Ingester` que emiten
`ProgressEvent` al canal `Progress`. `handleIngest` **ya está escrito** para consumir ese canal por
SSE, así que la web queda funcionando sola. `cmd/photos.go` se reduce a una cáscara que imprime el
progreso. Aprovechar para arreglar el emparejamiento de nombres y blindar el unzip.

**Etapa 2 — Endpoints que faltan.** `GET /api/photos/{id}/file` (original, para el Lightbox),
`POST /api/auth/logout`, `DELETE /api/photos/{id}` (recordar borrar también archivo y miniatura),
y decidir el contrato de `/api/albums` alineando `fetchAlbums`.

**Etapa 3 — Álbumes de verdad.** Poblar `albums` y `album_photos` durante la ingesta (cada carpeta
de Takeout es un álbum), añadir `AddPhoto`/`RemovePhoto` al repo, y darle contenido propio al ítem
del sidebar.

**Etapa 4 — Robustez.** `http.Server` con timeouts y `Shutdown(ctx)` grácil, rollback en
`App.Start`, validaciones en `App.New`, timeout de bootstrap de Tor, `slog` en lugar de los
`fmt.Println`, auto-rotación EXIF, y respetar `thumbHeight`.

**Etapa 5 — Desktop (bug 6)** y **Etapa 6 — tests de verdad** (usar `t.TempDir()` y el
`profile.NewWithBase` que ya existe justo para eso; poner el test de Tor detrás de `testing.Short()`).

`internal/uploads` puede quedarse como está hasta que todo lo anterior funcione.

---

## 12. Cómo probar el proyecto

### 12.1 Comprobaciones estáticas (segundos, hazlas siempre)

```bash
go build ./...      # ¿compila todo? Hoy: sí
go vet ./...        # errores comunes que el compilador no ve. Hoy: 0 hallazgos
gofmt -l .          # lista archivos mal formateados (vacío = todo bien)
go mod tidy         # ordena go.mod; hoy moverá goexif y wails al bloque directo
cd ui && npx tsc --noEmit && npm run lint
```

`go build` no deja binarios sueltos cuando le pasas un paquete (`./...`), sólo compila y descarta.

### 12.2 Tests de Go

Go trae el framework de tests en la librería estándar: un archivo `*_test.go` junto al código,
funciones `func TestAlgo(t *testing.T)`, y se ejecuta con `go test`.

```bash
go test ./...                              # todos los paquetes
go test ./internal/image/                  # sólo uno
go test -v -run TestGenerateThumbnail ./internal/image/   # un test concreto, verboso
go test -race ./...                        # detector de condiciones de carrera
go test -cover ./...                       # cobertura
go test -short ./...                       # salta los que consultan testing.Short()
make test                                  # = go test -v -race ./...
```

> ⚠️ **Hoy `make test` no es fiable.** Los dos tests existentes no tienen aserciones, usan rutas
> absolutas al home de este equipo, y `TestConnectToTOR` abre una conexión real a la red Tor
> (30–60 s, o cuelgue si no hay salida a internet). En otra máquina fallan. Ver bug #11.

**Cómo debería verse un test aquí.** El proyecto ya tiene lo necesario para hacerlos bien:
`t.TempDir()` da un directorio temporal que Go borra solo al terminar, y `profile.NewWithBase()`
existe justo para no tocar el `~/.allium` real:

```go
func TestProfileCreateAndLogin(t *testing.T) {
    mgr := profile.NewWithBase(t.TempDir())   // aislado, se limpia solo

    if err := mgr.Create("juan", "secreto123"); err != nil {
        t.Fatalf("Create falló: %v", err)     // Fatalf aborta este test
    }
    if !mgr.Exists("juan") {
        t.Error("el perfil debería existir")  // Error marca fallo pero sigue
    }
    if err := mgr.Login("juan", "incorrecta"); !errors.Is(err, profile.ErrInvalidPassword) {
        t.Errorf("esperaba ErrInvalidPassword, obtuve %v", err)
    }
}
```

Para la capa de BD puedes abrir SQLite **en memoria** y tener una base limpia por test:
`db.InitDB(filepath.Join(t.TempDir(), "test.db"))`.

`t.Fatalf` para cuando no tiene sentido continuar; `t.Errorf` para acumular fallos.
Las convenciones son: nombre `TestLoQueHace`, y si pruebas varios casos, un slice de structs
(«table-driven tests», el idioma más común en Go).

### 12.3 Prueba manual de punta a punta

Esto es lo que de verdad te dice si el producto funciona:

```bash
make build-node
./build/allium-node profile create demo      # te pedirá contraseña dos veces
./build/allium-node profile login demo
./build/allium-node add-photos ./tests/      # el servidor debe estar APAGADO
./build/allium-node start
./build/allium-node ps                       # ¿aparece "● corriendo"?
```

Y contra la API (necesitas `jq` para leer el JSON cómodamente):

```bash
curl -s -X POST 127.0.0.1:41110/api/auth/register \
     -d '{"username":"juan","password":"12345678"}'
TOKEN=$(curl -s -X POST 127.0.0.1:41110/api/auth/login \
     -d '{"username":"juan","password":"12345678"}' | jq -r .token)

curl -s -H "Authorization: Bearer $TOKEN" '127.0.0.1:41110/api/photos?limit=5' | jq '.photos[0]'
curl -s -H "Authorization: Bearer $TOKEN" 127.0.0.1:41110/api/status
curl -s -H "Authorization: Bearer $TOKEN" 127.0.0.1:41110/api/photos/1/thumb -o /tmp/t.webp
curl -N -H "Authorization: Bearer $TOKEN" -X POST 127.0.0.1:41110/api/ingest \
     -d '{"source_path":"./tests"}'          # -N = sin buffer, para ver el SSE en vivo
```

Ese primer `jq '.photos[0]'` es el que delata el **bug #1**: hoy verás `"ID"` y `"CapturedAt"`
en mayúsculas en lugar de `id` y `capturedAt`. Y `/api/status` te cerrará la conexión (**bug #2**).

Inspeccionar la base directamente ayuda mucho a saber si la ingesta funcionó:

```bash
sqlite3 ~/.allium/demo/data/allium.db 'select id,hash,title,captured_at from photos limit 5;'
sqlite3 ~/.allium/demo/data/allium.db 'select count(*) from photos;'
ls ~/.allium/demo/data/thumbs/ | head
tail -f ~/.allium/demo/server.log           # los errores del daemon salen aquí
```

Luego el navegador: `http://127.0.0.1:41110`, registrarse, y ver la galería. Con `--tor`, copiar
la `.onion` de la StatusBar y abrirla en Tor Browser (paciencia: el primer bootstrap tarda).

Para limpiar y empezar de cero: `./build/allium-node uninstall --force` (borra `~/.allium` entero).

### 12.4 Desarrollo con recarga automática

```bash
make dev-api    # air recompila el Go al guardar (.air.toml)
make dev-ui     # Vite con HMR en :5173
```

⚠️ Antes de usarlos juntos, arregla el proxy de `ui/vite.config.ts`: apunta a `localhost:8080` y
el servidor escucha en **41110** (bug #12).

### 12.5 Cómo depurar

- `log/slog` ya está en `core.App` (`a.log.Info(...)`). Es el logger estructurado estándar de Go;
  usa eso en vez de `fmt.Println` para cualquier traza nueva.
- Cuando el daemon no arranca, la respuesta está en `~/.allium/<perfil>/server.log`.
- `dlv` (Delve) es el debugger de Go: `dlv debug ./cmd/allium-node -- start`.
- `go test -race` es la única forma práctica de detectar carreras en el worker pool de la ingesta.

---

## 13. Go básico

Si vienes de otro lenguaje, esto es todo lo que necesitas para leer este repo. Los ejemplos son
código real del proyecto.

### Estructura del proyecto

- **`go.mod`** declara el nombre del módulo (`allium-server`) y las dependencias. Por eso los
  imports son `allium-server/internal/api`, no rutas relativas.
- **`cmd/<algo>/`** es la convención para binarios: cada subdirectorio con `package main` produce
  un ejecutable.
- **`internal/`** es especial **para el compilador**: nadie fuera de este módulo puede importar
  esos paquetes. Es privacidad a nivel de proyecto.
- Un **paquete** = un directorio. Todos los archivos de una carpeta comparten el mismo `package`
  y se ven entre sí sin importarse. Por eso `handlers.go` usa `writeJSON` de otro archivo sin ceremonia.
- **Mayúscula = público, minúscula = privado.** `func Start()` se puede usar desde otro paquete;
  `func writePID()` no. Aplica igual a tipos, campos de struct y métodos. Esto explica por qué
  `PhotoRepository.db` (minúscula) sólo es accesible dentro de `internal/storage`.

### Errores: se devuelven, no se lanzan

No hay excepciones. Las funciones devuelven `(resultado, error)` y **se comprueba siempre**:

```go
photo, err := s.photoRepo.GetByID(r.Context(), photoID)
if err != nil {
    return  // o manejarlo
}
```

`fmt.Errorf("... %w", err)` **envuelve** el error conservando el original, y
`errors.Is(err, sql.ErrNoRows)` pregunta si en algún punto de esa cadena está ese error concreto.
Ese par (`%w` + `errors.Is`) se usa por todo el repo:

```go
return nil, fmt.Errorf("Photo with id %d not found: %w", id, err)
...
if errors.Is(err, sql.ErrNoRows) { ... }
```

Los errores «centinela» se declaran como variables de paquete para poder compararlos:
`var ErrProfileNotFound = errors.New("perfil no encontrado")`.

### `defer`

Programa una llamada para cuando la función termine, pase lo que pase. Es como `finally`:

```go
openedFile, err := os.Open(filePath)
if err != nil { return "", err }
defer openedFile.Close()   // se ejecutará al salir, por cualquier camino
```

Se escribe justo después de abrir el recurso, para no olvidarlo. En `core.App` verás
`defer app.Stop()` y en la ingesta `defer database.Close()`.

### Punteros y receptores

`*Photo` es «puntero a Photo», `&photo` es «dirección de». Los métodos con receptor puntero
(`func (r *PhotoRepository) Save(...)`) evitan copiar el struct y permiten modificarlo.

**Un puntero puede ser `nil`, y usarlo revienta el programa (panic).** Eso es exactamente el
bug #2 y el #4 de este proyecto: `s.torCtrl.OnionAddress()` donde `torCtrl` es `nil`. El arreglo
idiomático es comprobar antes: `if c.tor != nil { ... }`, como sí hace `tor.Controller.Stop()`.

### Structs y tags

Los tags son metadatos en texto que leen las librerías por reflexión. Aquí conviven dos:

```go
type Photo struct {
    bun.BaseModel `bun:"table:photos,alias:p"`
    ID   int64  `bun:"id,pk,autoincrement"`
    Hash string `bun:"hash,unique,not null"`
}
```

`bun:` lo lee el ORM para saber la tabla y las columnas. **`json:` lo lee `encoding/json`** para
saber cómo nombrar los campos al serializar — y como aquí no hay ninguno, la API responde con los
nombres de Go (`"ID"`, `"Hash"`). Ese es el bug #1. Con `json:"id"` respondería `"id"`.

`bun.BaseModel` embebido sin nombre de campo es **embedding**: hereda comportamiento sin herencia
de clases. Go no tiene clases ni herencia; tiene composición.

### Interfaces

Se satisfacen implícitamente: si tu tipo tiene los métodos, cumple la interfaz, sin declararlo.
Por eso `writeJSON(w http.ResponseWriter, ...)` acepta cualquier cosa que sepa escribir una
respuesta, y por eso el SSE puede preguntar en tiempo de ejecución si el writer sabe hacer flush:

```go
flusher, ok := w.(http.Flusher)   // type assertion: ¿este writer también es un Flusher?
if !ok { ... }
```

`any` (alias de `interface{}`) es «cualquier tipo» — lo verás en `writeJSON(w, status, v any)`.

### Concurrencia: goroutines, canales, WaitGroup

Una **goroutine** es un hilo ligerísimo; se lanza con `go`:

```go
go func() {
    if err := s.server.Start(); err != nil { ... }
}()
```

Un **canal** comunica goroutines de forma segura. El worker pool de la ingesta
(`cmd/allium-node/cmd/photos.go`) es el ejemplo canónico y vale la pena leerlo entero:

```go
jobs := make(chan *FileGroup, len(groups))   // canal con buffer
results := make(chan Result, len(groups))
var wg sync.WaitGroup                        // contador para esperar a todos

for range numWorkers {                       // 8 trabajadores
    wg.Add(1)
    go func() {
        defer wg.Done()                      // avisa al terminar
        for group := range jobs {            // consume hasta que jobs se cierre
            results <- Result{...}
        }
    }()
}

for _, g := range groups { jobs <- g }       // repartir trabajo
close(jobs)                                  // señal de "no hay más"
wg.Wait()                                    // esperar a los 8
close(results)
for res := range results { ... }             // recoger resultados
```

Reglas prácticas: **cierra el canal quien escribe**, nunca quien lee. `range` sobre un canal
termina cuando lo cierran. `wg.Add` antes de lanzar, `defer wg.Done()` dentro.

El otro patrón del repo es el **semáforo**: `workerPool chan struct{}` con capacidad N en
`downloads.Ingester` limita cuántas goroutines corren a la vez (`struct{}` no ocupa memoria).

### `context.Context`

Se propaga como **primer parámetro** por convención y sirve para cancelar. En este proyecto:

- `r.Context()` en un handler HTTP se cancela solo si el cliente cierra la conexión — por eso las
  consultas a BD lo reciben y se abortan en vez de seguir trabajando para nadie.
- `signal.NotifyContext(ctx, SIGINT, SIGTERM)` en `runDaemon` crea un contexto que se cancela al
  recibir la señal; `<-ctx.Done()` bloquea el programa hasta entonces.
- **Nunca uses `context.Background()` dentro de un handler** cuando tienes `r.Context()` a mano;
  hay dos sitios en el repo que lo hacen mal (`api/server.go` y `api/auth.go`).

### Cosas de sintaxis que confunden al principio

- `:=` declara e infiere el tipo; `=` sólo asigna. Dentro de una función casi siempre `:=`.
- **Variable declarada y no usada = error de compilación.** No es un warning. Usa `_` para
  descartar: `lat, lon, _ := x.LatLong()`.
- `for i := range 8 {}` (Go 1.22+) itera 8 veces. `for k, v := range mapa {}` recorre un mapa
  **en orden aleatorio** — nunca dependas del orden.
- `slice = append(slice, x)` — `append` devuelve el slice, hay que reasignarlo.
- Un `map` no inicializado es nil y escribir en él revienta: `groups := map[string]*FileGroup{}`.
- `switch` no necesita `break`, y puede ir sin condición (`switch { case d < time.Minute: ... }`),
  como en `formatUptime`.
- `//go:embed` es una **directiva de compilación**, no un comentario: mete archivos dentro del
  binario. Debe ir pegada a la variable, sin línea en blanco.
- `init()` se ejecuta automáticamente al cargar el paquete — así se registran los subcomandos
  de Cobra.

### Comandos del día a día

```bash
go run ./cmd/allium-node start   # compilar y ejecutar sin dejar binario
go build -o build/x ./cmd/...    # compilar a un archivo
go get github.com/algo/lib       # añadir dependencia
go mod tidy                      # limpiar go.mod/go.sum
go doc net/http.ServeMux         # documentación en la terminal
GOOS=linux GOARCH=arm64 go build # cross-compile (ver la nota de CGO en §2)
```

`go doc` y https://pkg.go.dev son la referencia; la documentación de un paquete son literalmente
los comentarios sobre cada función exportada. Este repo los tiene bastante completos.

---

## 14. Glosario de decisiones

Cosas que parecen raras hasta que sabes por qué:

- **Dos sistemas de contraseñas.** `internal/profile` autentica *quién usa esta computadora*
  (perfiles del sistema, hash en `profile.json`); `internal/api/auth.go` autentica *quién accede
  a la galería web* (tabla `users`). Son independientes a propósito: un perfil puede tener varios
  usuarios web.
- **Driver SQLite puro Go** para poder cross-compilar a Raspberry Pi sin toolchain de C… pero
  **govips sí usa CGO**, así que en la práctica el cross-compile de `build-node-arm` necesita
  libvips para ARM. Es una contradicción heredada que hay que resolver algún día (o aceptar CGO,
  o buscar un redimensionador puro Go).
- **Daemonización a mano** (re-exec + `Setsid`) en lugar de systemd: funciona igual en Linux, macOS
  y cualquier Unix, sin pedirle nada al usuario ni requerir root.
- **El hash es la identidad.** Deduplicación, nombre del archivo en disco y nombre de la miniatura
  son todos el mismo SHA256. Importar dos veces la misma foto es un no-op.
- **`captured_at` como `int64`** en vez de `time.Time`: ordenar la galería sin parsear fechas.
- **Tokens en BD en vez de JWT:** para poder invalidarlos (aunque el endpoint de logout aún no exista).
- **El frontend se embebe en el binario** (`go:embed`), así que el binario compilado es
  autosuficiente: no hay que copiar una carpeta `static/` junto a él.
- **`GetByHash` devuelve `nil, nil`** cuando no encuentra nada — no es un error, es la señal de
  "esta foto es nueva".
- **Router estándar de Go 1.22+**: `"GET /api/photos/{id}"` y `r.PathValue("id")`. No hace falta
  chi, gorilla ni gin.



Tres hallazgos que conviene que sepas ya, porque explican por qué la galería sale vacía aunque la BD tenga fotos:

Los modelos no tienen tags json:, así que la API responde {"ID":1,"CapturedAt":…} y el frontend espera {id, capturedAt} → photo.id es undefined y todas las miniaturas piden /api/photos/undefined/thumb.
GET /api/status hace panic: core.App.Start() crea el Processor y el tor.Controller pero nunca se los pasa a api.NewServer, y handleStatus desreferencia ese nil.
No hay ninguna ruta para importar fotos con el servidor encendido: /api/ingest depende de downloads.Ingester, que es un stub, y add-photos se rehúsa a correr si el daemon está activo.
Un detalle del que quizá no te habías dado cuenta: los .json de Google Takeout nunca se emparejan con su foto — retrieveGroups agrupa por TrimSuffix(base, ext), que para IMG_1234.jpg.supplemental-metadata.json deja IMG_1234.jpg.supplemental-metadata en vez de IMG_1234. Toda la ingesta cae al fallback de EXIF y se pierden título, descripción y GPS.