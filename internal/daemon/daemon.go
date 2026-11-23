// Package daemon runs the server as a background process, one per user folder.
package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PIDPath is the file where the background server writes its process id.
func PIDPath(userFolder string) string { return filepath.Join(userFolder, "server.pid") }

// StopPath is the file Terminate creates on Windows to ask the server to stop.
func StopPath(userFolder string) string { return filepath.Join(userFolder, "server.stop") }

// LockPath is the file the background server keeps locked while it runs.
func LockPath(userFolder string) string { return filepath.Join(userFolder, "server.lock") }

// held is the lock of this process; the OS frees it when the process dies, even in a crash.
var held *os.File

// Acquire marks this process as the server of the user folder, and fails if another one already is.
func Acquire(userFolder string) error {
	f, err := os.OpenFile(LockPath(userFolder), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := tryLock(f); err != nil {
		f.Close()
		return fmt.Errorf("another server is already running: %w", err)
	}
	if err := os.WriteFile(PIDPath(userFolder), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		f.Close()
		return err
	}
	held = f
	return nil
}

// Release frees the lock and removes the pid file; call it when the server exits.
func Release(userFolder string) {
	os.Remove(PIDPath(userFolder))
	if held != nil {
		held.Close()
		held = nil
	}
}

// Running tells if the background server of this user is alive. It checks the
// lock, not the pid, so a recycled pid after a crash cannot fool it.
func Running(userFolder string) (pid int, ok bool) {
	f, err := os.OpenFile(LockPath(userFolder), os.O_RDWR, 0600)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	if tryLock(f) == nil { // nobody holds it: the server is not running
		os.Remove(PIDPath(userFolder))
		return 0, false
	}
	// Acquire writes the pid right after locking, so wait a moment for it.
	for i := 0; i < 10; i++ {
		if data, err := os.ReadFile(PIDPath(userFolder)); err == nil {
			if pid, err = strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				return pid, true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, true
}

// WaitStopped waits until the server is gone and tells if that happened before the timeout.
func WaitStopped(userFolder string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, ok := Running(userFolder); !ok {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// Child is a background server that was just launched.
type Child struct {
	// Lines is everything the child prints (stdout and stderr), closed when it exits.
	Lines  <-chan string
	cmd    *exec.Cmd
	exited chan struct{}
}

// Spawn runs this same program in the background with args and sends it the KEK
// through its stdin, so it never shows in the process list.
func Spawn(kek []byte, args ...string) (*Child, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("finding executable: %w", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = w, w
	detach(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.Close()
		w.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, fmt.Errorf("starting background process: %w", err)
	}
	w.Close() // the child keeps its own copy

	_, err = stdin.Write(kek)
	stdin.Close()
	if err != nil {
		cmd.Process.Kill()
		r.Close()
		return nil, fmt.Errorf("sending key to background process: %w", err)
	}

	lines := make(chan string)
	go func() {
		defer close(lines)
		defer r.Close()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	exited := make(chan struct{})
	go func() { // reaps the child if it dies while we still run
		cmd.Wait()
		close(exited)
	}()

	return &Child{Lines: lines, cmd: cmd, exited: exited}, nil
}

// Stop asks the child to shut down cleanly and kills it if it takes more than
// 15 seconds. userFolder is needed for the Windows stop file.
func (c *Child) Stop(userFolder string) {
	Terminate(userFolder, c.cmd.Process.Pid)
	select {
	case <-c.exited:
	case <-time.After(15 * time.Second):
		c.cmd.Process.Kill()
	}
}

// WatchStop calls stop when the stop file shows up (Windows has no SIGTERM between processes).
func WatchStop(userFolder string, done <-chan struct{}, stop func()) {
	os.Remove(StopPath(userFolder)) // a leftover file must not stop a new server
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if _, err := os.Stat(StopPath(userFolder)); err == nil {
				os.Remove(StopPath(userFolder))
				stop()
				return
			}
		}
	}
}
