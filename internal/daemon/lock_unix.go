//go:build !windows

package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// acquireSingleton abre un lock exclusivo no-bloqueante sobre un archivo
// convencional. Si otro daemon ya lo tiene, falla: así garantizamos que
// nunca corran dos daemons simultáneos aunque el cliente dispare el spawn
// en paralelo o queden PID huérfanos.
//
// El file descriptor devuelto debe permanecer abierto durante toda la vida
// del daemon; al cerrarse se libera el lock automáticamente.
func acquireSingleton() (*os.File, error) {
	infoP, err := infoPath()
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(filepath.Dir(infoP), "navtunnel-cli.lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("ya hay otro daemon corriendo")
		}
		return nil, err
	}
	return f, nil
}
