//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// acquireSingleton en Windows usa CREATE_NEW (O_EXCL): si el archivo ya
// existe, falla. El daemon escribe su PID; al salir se borra el archivo.
// No es tan robusto como flock ante crashes, pero el caller verifica
// processAlive() del PID en el InfoFile antes de conectarse y sobrescribe
// si está muerto.
func acquireSingleton() (*os.File, error) {
	infoP, err := infoPath()
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(filepath.Dir(infoP), "navtunnel-cli.lock")

	// Intento exclusivo. Si existe, fallamos.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0o600)
	if err != nil {
		// Revisar si el lock es de un proceso muerto: si sí, lo borramos y
		// reintentamos una vez.
		if existing, rerr := os.ReadFile(lockPath); rerr == nil {
			var pid int
			_, _ = fmt.Sscanf(string(existing), "%d", &pid)
			if pid > 0 && !processAlive(pid) {
				_ = os.Remove(lockPath)
				return os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0o600)
			}
		}
		return nil, errors.New("ya hay otro daemon corriendo")
	}
	return f, nil
}
