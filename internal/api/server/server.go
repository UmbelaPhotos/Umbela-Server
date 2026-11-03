package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"umbela-server/internal/api/interceptors"
	"umbela-server/internal/api/pb/pbconnect"
	"umbela-server/internal/auth"
	"umbela-server/internal/config"
	"umbela-server/internal/tor"
	"umbela-server/internal/vault"
)

// Run arranca el servidor HTTP/2, Tor y expone los RPCs
func Run(ctx context.Context, cfg *config.ServerConfig, v *vault.Vault, torCtrl *tor.Controller) error {
	// 1. Iniciar Almacenes en RAM
	sessionStore := auth.NewSessionStore()
	keyAgent := auth.NewKeyAgent()

	// 2. Configurar Interceptor (requiere Bearer token en RAM)
	authInterceptor := interceptors.NewAuthInterceptor(sessionStore)
	connectOpts := connect.WithInterceptors(authInterceptor)

	// 3. Crear los Servidores (handlers) inyectando dependencias
	authHandlerImpl := NewAuthServer(v, sessionStore, keyAgent, torCtrl)
	photoHandlerImpl := NewPhotoServer(v, keyAgent)

	// 4. Montar Enrutador (Mux)
	mux := http.NewServeMux()

	authPath, authHandler := pbconnect.NewAuthServiceHandler(authHandlerImpl, connectOpts)
	photoPath, photoHandler := pbconnect.NewPhotoServiceHandler(photoHandlerImpl, connectOpts)

	mux.Handle(authPath, authHandler)
	mux.Handle(photoPath, photoHandler)

	// 5. Configurar HTTP/2 nativo sin TLS (h2c)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	httpServer := &http.Server{
		Handler:   mux,
		Protocols: protocols,
	}

	// 6. Arrancar Tor
	if err := torCtrl.Start(ctx); err != nil {
		return err
	}
	defer torCtrl.Stop()

	// Start with bine's listener
	listener := torCtrl.LocalListener()
	fmt.Printf("Tor service listening on: %s\n", torCtrl.OnionAddress())

	// 8. Apagado Limpio y Destrucción de Claves
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("http error: %v\n", err)
		}
	}()

	// Face recognition runs in the background while the server is up. If it
	// cannot start (for example the ONNX Runtime library is missing) the rest
	// of the server keeps working.
	scanCtx, stopScan := context.WithCancel(ctx)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		if err := v.RunFaceScanner(scanCtx); err != nil && scanCtx.Err() == nil {
			fmt.Printf("Face recognition is off: %v\n", err)
		}
	}()

	<-stopChan
	fmt.Println("\nShutting down server and removing RAM...")

	stopScan()
	select {
	case <-scanDone:
	case <-time.After(10 * time.Second):
	}

	keyAgent.ClearKey()
	sessionStore.DestroyAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
