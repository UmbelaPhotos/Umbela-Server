# ¿Cómo funcionan los Comandos CLI con esta estructura?

Así fluirá el código para cada comando dentro de cmd/umbela-node/cmd/:
**Por cada comando que pida contraseña, la masterkey se guardara en ram para evitar preguntar una y otra vez la contraseña en cada comando**

## A. umbela-server create (create.go)
- Pide una contraseña nueva al usuario por terminal.
- Llama a crypto.GenerateSalt().
- Llama a crypto.DeriveMasterKey(password, salt).
- Llama a vault.CreateVault(salt, config, masterKey) para crear los archivos hermanos.
- Llama a db.InitSQLCipher(...) para generar el .db vacío y correr migraciones.
- Llama a tor.GenerateHiddenService() [One hop] para crear el .onion inicial.

## B. umbela-server <user> start (start.go)
(Inicia el servidor completo para atender a la app móvil)
- Llama a db.InitSQLCipher(...) con la MasterKey.
- Inicia Govips y levanta el procesador image.NewProcessor().
- Levanta api.NewServer(...) (gRPC) y tor.Start().
- Se queda ejecutando (Daemon) esperando conexiones de Tor.

## C. umbela-server <user> add <ruta> (add.go)
(Proceso batch local sin levantar la red)
- Abre db.InitSQLCipher.
- Levanta GoVips image.NewProcessor().
- Llama a pipeline.IngestDirectory(ruta, masterKey, db, processor).
- Termina y se cierra. (No levanta Tor ni gRPC).

## D. umbela-server <user> pair (pair.go)
(Para conectar el teléfono al nodo)
- Lee el .onion de tor.GetOnionAddress().
- Pide al SessionManager crear un Pairing Token efímero.
- Imprime en la terminal un Código QR o link con: allium://pair?onion=...&token=...
- Levanta un mini-servidor temporal o espera a que el teléfono se conecte para completar el handshake.



``` sh
📂 allium-server
├── 📁 cmd/
│   ├── 📁 umbela-desktop/    # (Futuro) Entrypoint para la UI gráfica (Wails/Electron/webview)
│   └── 📁 umbela-node/       # Entrypoint del servidor headless y CLI
│       ├── 📄 main.go        # Solo llama a cmd.Execute()
│       └── 📁 cmd/           # Comandos de Cobra CLI
│           ├── 📄 root.go
│           ├── 📄 create.go  # Lógica para 'umbela-server create'
│           ├── 📄 add.go     # Lógica para 'umbela-server <user> add <ruta>'
│           ├── 📄 pair.go    # Lógica para 'umbela-server <user> pair'
│           └── 📄 start.go   # Lógica para 'umbela-server <user> start'
├── 📁 internal/
│   ├── 📁 api/               # Todo lo relacionado a red y gRPC
│   │   ├── 📁 proto/         # Archivos .proto
│   │   ├── 📁 pb/            # Código autogenerado por protoc
│   │   ├── 📁 server/        # Servidor gRPC y Handlers (PhotosService, AuthService)
│   │   └── 📁 interceptors/  # Middleware: auth_interceptor.go (valida tokens gRPC)
│   │
│   ├── 📁 auth/              # Gestión de Sesiones y RAM (El corazón Zero-Knowledge)
│   │   ├── 📄 key_agent.go   # Mantiene MasterKey en RAM, TTL de 15 min, memzero
│   │   └── 📄 session.go     # SessionManager: mapea SessionToken -> UserHash + MasterKeyRef
│   │
│   ├── 📁 vault/             # Gestión de la Bóveda en Disco (Identidad/Configuración)
│   │   ├── 📄 manager.go     # Lee/Escribe el .salt y el .config.enc
│   │   └── 📄 setup.go       # Lógica para crear una bóveda nueva por primera vez
│   │
│   ├── 📁 crypto/            # Primitivas criptográficas (Lo que ya tienes)
│   │   ├── 📄 aes_gcm.go
│   │   ├── 📄 argon2id.go
│   │   └── 📄 hmac_sha256.go
│   │
│   ├── 📁 db/                # Exclusivo para base de datos
│   │   ├── 📄 connection.go  # InitSQLCipher()
│   │   └── 📄 migrations/    # Esquemas de tablas (bun)
│   │
│   ├── 📁 image/             # Procesamiento local
│   │   └── 📄 processor.go   # govips y blurhash
│   │
│   ├── 📁 pipeline/          # (NUEVO) Flujo de ingestión de fotos
│   │   └── 📄 ingester.go    # Escanea directorio -> govips -> crypto -> db
│   │
│   ├── 📁 models/            # Structs de BD, Configuración y Respuestas
│   │   ├── 📄 album.go
│   │   ├── 📄 photo.go
│   │   └── 📄 config.go      # Struct de preferencias del usuario
│   │
│   ├── 📁 storage/           # Patrón Repositorio (Consultas SQL con Bun)
│   │   ├── 📄 photo_repo.go
│   │   └── 📄 album_repo.go
│   │
│   └── 📁 tor/
│       └── 📄 controller.go  # Arranca servicio oculto, guarda claves en DataDir
│
├── 📄 go.mod
└── 📄 README.md
```