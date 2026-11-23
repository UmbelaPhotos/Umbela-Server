//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const detachedProcess = 0x00000008

// detach runs the child with no console, so closing the terminal does not kill it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
		HideWindow:    true,
	}
}

// Init does nothing on Windows: a broken pipe is only an error there.
func Init() {}

var procLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

// tryLock takes an exclusive lock on f without waiting; closing f frees it.
func tryLock(f *os.File) error {
	const exclusiveFailImmediately = 0x3 // LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	var ol syscall.Overlapped
	r, _, err := procLockFileEx.Call(f.Fd(), exclusiveFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

// Terminate asks the server to shut down cleanly by creating the stop file it watches.
func Terminate(userFolder string, pid int) error {
	return os.WriteFile(StopPath(userFolder), nil, 0600)
}
