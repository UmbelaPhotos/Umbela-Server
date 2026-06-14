# Makefile para Allium 🧅
# Atajos de compilación cruzada para ambos binarios del proyecto.
#
# Uso:
#   make build-node      → compila el binario CLI para linux/amd64
#   make build-node-arm  → compila el binario CLI para linux/arm64 (Raspberry Pi 4)
#   make build-desktop   → compila el binario GUI (requiere Wails instalado)
#   make dev-api         → arranca el servidor API en modo desarrollo con live-reload
#   make dev-ui          → arranca el servidor de desarrollo de Vite (React)
#   make test            → corre todos los tests de Go
#   make clean           → borra los binarios compilados

BINARY_NODE    := allium-node
BINARY_DESKTOP := allium-desktop
BUILD_DIR      := ./build
NODE_PKG       := ./cmd/allium-node
DESKTOP_PKG    := ./cmd/allium-desktop

# Versión inyectada en tiempo de compilación via ldflags
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS        := -ldflags "-X main.Version=$(VERSION) -s -w"

.PHONY: all build-node build-node-arm build-desktop dev-api dev-ui test clean tidy

all: build-node build-desktop

## build-ui: Compilar el frontend React (requiere Node.js)
build-ui:
	@echo "→ Compilando UI React..."
	cd ui && npm run build
	@echo "✓ internal/ui/dist/"

## build-node: Compilar binario CLI para linux/amd64 (incluye UI embebida)
build-node: build-ui
	@echo "→ Compilando $(BINARY_NODE) para linux/amd64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NODE) $(NODE_PKG)
	@echo "✓ $(BUILD_DIR)/$(BINARY_NODE)"

## build-node-arm: Compilar binario CLI para Raspberry Pi (linux/arm64)
build-node-arm: build-ui
	@echo "→ Compilando $(BINARY_NODE) para linux/arm64 (Raspberry Pi)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NODE)-arm64 $(NODE_PKG)
	@echo "✓ $(BUILD_DIR)/$(BINARY_NODE)-arm64"

## build-desktop: Compilar binario GUI con Wails
build-desktop:
	@echo "→ Compilando $(BINARY_DESKTOP) con Wails..."
	@command -v wails >/dev/null 2>&1 || { echo "⚠ Wails no está instalado. Instalar con: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	go build -tags production $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_DESKTOP) $(DESKTOP_PKG)
	@echo "✓ $(BUILD_DIR)/$(BINARY_DESKTOP)"

## dev-api: Servidor API con live-reload usando Air (go install github.com/air-verse/air@latest)
dev-api:
	@command -v air >/dev/null 2>&1 || { echo "⚠ Air no está instalado. Instalar con: go install github.com/air-verse/air@latest"; exit 1; }
	air -c .air.toml

## dev-ui: Servidor de desarrollo de Vite React
dev-ui:
	@echo "→ Iniciando servidor de desarrollo UI..."
	cd ui && npm run dev

## dev: Arranca API y UI en paralelo (requiere GNU Make)
dev:
	@echo "→ Iniciando modo desarrollo completo..."
	$(MAKE) -j2 dev-api dev-ui

## test: Correr todos los tests de Go
test:
	go test -v -race ./...

## tidy: Limpiar y ordenar dependencias de Go
tidy:
	go mod tidy

## clean: Borrar binarios compilados
clean:
	rm -rf $(BUILD_DIR)
	@echo "✓ Limpieza completa"

# Help automático: parsea los comentarios ## para generar ayuda
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
