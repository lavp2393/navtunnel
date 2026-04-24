//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

const isUnix = false

// detachFromParent en Windows usa DETACHED_PROCESS + CREATE_NEW_PROCESS_GROUP
// para que el hijo no comparta la consola del cliente ni muera con él.
func detachFromParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
}

// processAlive en Windows usa os.FindProcess (que en Win32 abre un handle al
// PID) + Signal nil; si el proceso murió, FindProcess puede devolver nil
// pero Signal fallará. Best-effort — el control real está en el TCP ping.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
