//go:build !windows

package daemon

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// detach puts the child in its own session so closing the terminal does not kill it.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// Init must be called by the background process: once the parent is gone its
// output pipe is broken, and without this a print would kill the server.
func Init() { signal.Ignore(syscall.SIGPIPE) }

// tryLock takes an exclusive lock on f without waiting; closing f frees it.
func tryLock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }

// Terminate asks the server to shut down cleanly (SIGTERM).
func Terminate(userFolder string, pid int) error {
	if pid <= 0 {
		return errors.New("unknown server pid")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
