//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

const isUnix = true

// detachFromParent separa al daemon del grupo de procesos del cliente.
// En modo consola conserva la sesión TTY para sudo; en otros casos crea una
// sesión nueva y elimina la dependencia de la terminal.
func detachFromParent(cmd *exec.Cmd, consoleTTY bool) {
	if consoleTTY {
		// A new process group separates the daemon from the shell's foreground
		// job while retaining its session tty for sudo's tty-scoped ticket.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		return
	}
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
