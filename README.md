# 🧅 Allium: Servidor Soberano de Fotos Privadas

Allium es un ecosistema de software escrito en **Go** diseñado para convertir cualquier computadora (desde una Raspberry Pi hasta una PC de escritorio) en un servidor privado de fotos. El sistema ingiere datos de Google Takeout, los organiza localmente y los sirve de forma segura exclusivamente a través de la **Red Tor**.

---

## 🎯 Objetivos del Proyecto

*   **Soberanía de Datos:** Los archivos nunca tocan la "nube". Se quedan físicamente en el hardware del usuario.
*   **Privacidad de Red:** Acceso remoto seguro y exclusivo vía Tor Hidden Services (sin apertura de puertos, IPs públicas, ni configuración de NAT traversal).
*   **Eficiencia en Hardware Modesto:** Optimizado para correr 24/7 en equipos antiguos o Raspberry Pi mediante el uso de `libvips` y un sistema controlado de concurrencia (Worker Pool).
*   **Dualidad de Interfaz:** Un núcleo unificado (Core) que alimenta tanto una versión **Headless (CLI)** para servidores como una **Desktop (GUI)** para usuarios finales.

---

## 🛠️ Stack Tecnológico

*   **Lenguaje:** Go (Golang) 1.22+
*   **Base de Datos / ORM:** SQLite mediante **Bun ORM** utilizando el driver nativo `modernc.org/sqlite` (100% Pure Go, sin dependencias de CGO para máxima portabilidad).
*   **Procesamiento de Imagen:** libvips (vía `govips`) para la generación ultra rápida de thumbnails y Blurhash.
*   **Red Anónima:** Tor Daemon integrado mediante controladores de Go (ej. `cretz/bine`) para automatizar la creación del servicio oculto `.onion` sin intervención del usuario.
*   **Interfaces:**
    *   **CLI:** Cobra CLI (Comandos estructurados para terminal).
    *   **GUI:** Wails (Frontend moderno en React/Svelte/Tailwind + Backend nativo en Go).

---

## 📂 Estructura de Directorios (Monorepo)

```text
allium/
├── cmd/
│   ├── allium-node/     # Binario CLI (Servidor puro / Headless para Raspberry Pi)
│   └── allium-desktop/  # Binario GUI (Ventana Wails con soporte de comandos CLI)
├── internal/
│   ├── core/            # El "Cerebro" que coordina todo o como lo organices
│   ├── api/             # Servidor REST (servido localmente y expuesto a la red Tor)
│   ├── db/              # Inicialización de Bun ORM y configuración de SQLite (WAL mode)
│   ├── image/           # Procesamiento de imágenes (Thumbnails, WebP y Blurhash)
│   ├── tor/             # Controlador para iniciar y gestionar el proceso de Tor (.onion)
│   ├── downloads/       # Donde se gestiona lo del Takeout y cuando quieren añadir una foto o una carpeta 
│   ├── uploads/         # Este ignoralo por ahora pero va a ser el que va a hacer takeouts por si se quieren ir a google  
│   └── models/          # Structs de Go compartidos (Photo, User, Config mapped por Bun)
├── ui/                  # Código frontend de la galería web (React/Tailwind)
├── scripts/             # Utilidades de automatización y despliegue
├── go.mod               # Gestión de módulos de Go
└── Makefile             # Atajos para compilación cruzada de ambos binarios