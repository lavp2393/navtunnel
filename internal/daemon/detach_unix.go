//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

const isUnix = true

// detachFromParent deja al daemon sobreviviendo al cliente: nueva session
// (Setsid) para que no herede el tty y no reciba SIGHUP cuando el padre sale.
func detachFromParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// processAlive en Unix usa signal 0 directo sobre el PID, independiente de
// si el proceso es nuestro hijo o ya fue liberado con Process.Release().
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
