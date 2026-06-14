// Package daemon gestiona el ciclo de vida del servidor allium-node
// ejecutándose en segundo plano (background process).
//
// Estrategia: el binario se re-ejecuta a sí mismo con la flag --daemon-mode,
// que inhibe el fork y hace que el proceso hijo corra el servidor directamente.
// El proceso padre registra el PID del hijo y sale.
//
// Esto funciona sin dependencias externas (no requiere systemd, launchd ni screen)
// y es compatible con Linux, macOS y cualquier Unix.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FlagDaemonMode es la flag interna que indica que el proceso actual ES el daemon.
// No debe usarse directamente por el usuario; es manejada internamente.
const FlagDaemonMode = "--daemon-mode"

// PIDInfo contiene información sobre una instancia del servidor en ejecución.
type PIDInfo struct {
	ProfileName string
	PID         int
	PIDFile     string
	StartedAt   time.Time
	Running     bool
}

// Start lanza el servidor de un perfil en background.
// Re-ejecuta el binario actual con --daemon-mode y registra el PID.
//
// Input:
//   - profileName string — nombre del perfil activo
//   - pidFile string     — ruta donde guardar el PID
//   - extraArgs []string — argumentos adicionales para pasar al daemon
//     (ej: --port, --data-dir, etc.)
//
// Output: error si el proceso no se pudo lanzar
func Start(profileName, pidFile string, extraArgs []string) error {
	// Verificar si ya hay una instancia corriendo para este perfil
	if info, err := ReadPID(profileName, pidFile); err == nil && info.Running {
		return fmt.Errorf("el servidor del perfil %q ya está corriendo (PID %d)", profileName, info.PID)
	}

	// Preparar argumentos: el binario se re-ejecuta con daemon-mode
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo determinar la ruta del binario: %w", err)
	}

	args := append([]string{FlagDaemonMode, "--profile", profileName}, extraArgs...)

	// Abrir archivo de log para el daemon
	logDir := filepath.Dir(pidFile)
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "server.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600,
	)
	if err != nil {
		// Si no podemos crear el log, redirigir a /dev/null
		logFile, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	defer logFile.Close()

	cmd := exec.Command(self, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Desconectar el proceso del terminal padre para que sobreviva al cierre de sesión
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Crear nueva sesión → no recibe SIGHUP
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("fallo al lanzar el servidor en background: %w", err)
	}

	// Guardar el PID del proceso hijo
	pid := cmd.Process.Pid
	if err := writePID(pidFile, profileName, pid); err != nil {
		// El proceso ya inició; intentar matarlo para no dejarlo huérfano
		_ = cmd.Process.Kill()
		return fmt.Errorf("guardando PID file: %w", err)
	}

	// Breve espera para detectar fallos inmediatos de inicio
	time.Sleep(300 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(pidFile)
		return fmt.Errorf("el servidor falló al iniciar (revisa %s/server.log)", logDir)
	}

	// Liberar el proceso para que viva independientemente
	_ = cmd.Process.Release()

	return nil
}

// Stop envía SIGTERM al servidor del perfil y espera a que termine.
//
// Input:
//   - pidFile string — ruta al PID file del perfil
//
// Output: error si no hay servidor corriendo o no se pudo detener
func Stop(pidFile string) error {
	info, err := ReadPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("leyendo PID file: %w", err)
	}
	if !info.Running {
		// Limpiar el PID file obsoleto
		_ = os.Remove(pidFile)
		return fmt.Errorf("no hay servidor corriendo para ese perfil (PID file obsoleto limpiado)")
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("no se pudo encontrar el proceso %d: %w", info.PID, err)
	}

	// Enviar SIGTERM para apagado limpio
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("enviando SIGTERM al PID %d: %w", info.PID, err)
	}

	// Esperar hasta 10 segundos a que el proceso termine
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			// El proceso ya no existe → terminó limpiamente
			_ = os.Remove(pidFile)
			return nil
		}
	}

	// Si no terminó, enviar SIGKILL
	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(pidFile)
	return fmt.Errorf("el servidor (PID %d) no respondió a SIGTERM; se envió SIGKILL", info.PID)
}

// ReadPID lee el PID file de un perfil y verifica si el proceso está corriendo.
func ReadPID(profileName, pidFile string) (*PIDInfo, error) {
	info, err := ReadPIDFile(pidFile)
	if err != nil {
		return nil, err
	}
	info.ProfileName = profileName
	return info, nil
}

// ReadPIDFile lee un PID file y verifica si el proceso sigue vivo.
func ReadPIDFile(pidFile string) (*PIDInfo, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(parts) < 1 {
		return nil, fmt.Errorf("PID file malformado")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("PID inválido en %s: %w", pidFile, err)
	}

	var profileName string
	var startedAt time.Time
	if len(parts) >= 2 {
		profileName = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		_ = startedAt.UnmarshalText([]byte(strings.TrimSpace(parts[2])))
	}

	// Verificar si el proceso existe sin enviarte señal (kill -0)
	proc, _ := os.FindProcess(pid)
	running := false
	if proc != nil {
		err := proc.Signal(syscall.Signal(0))
		running = err == nil
	}

	return &PIDInfo{
		ProfileName: profileName,
		PID:         pid,
		PIDFile:     pidFile,
		StartedAt:   startedAt,
		Running:     running,
	}, nil
}

// IsRunning reporta si el servidor de un perfil está activo.
func IsRunning(pidFile string) bool {
	info, err := ReadPIDFile(pidFile)
	if err != nil {
		return false
	}
	return info.Running
}

// IsDaemonMode reporta si el proceso actual fue lanzado como daemon.
// Llamar en main() para bifurcar la lógica.
func IsDaemonMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == FlagDaemonMode {
			return true
		}
	}
	return false
}

// writePID escribe el PID y metadatos al PID file.
func writePID(pidFile, profileName string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0700); err != nil {
		return err
	}
	startedAt, _ := time.Now().MarshalText()
	content := fmt.Sprintf("%d\n%s\n%s\n", pid, profileName, startedAt)
	return os.WriteFile(pidFile, []byte(content), 0600)
}
