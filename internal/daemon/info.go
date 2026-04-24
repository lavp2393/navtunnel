package daemon

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// InfoFile contiene los datos que el daemon publica para que el cliente lo
// encuentre: puerto local y cookie de autenticación. Se escribe en un lugar
// convencional por OS y se borra al salir.
type InfoFile struct {
	Port  int
	Token string
	PID   int
}

// infoPath devuelve la ruta al archivo de info del daemon según la plataforma.
func infoPath() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "linux":
		dir = os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".cache", "navtunnel-cli")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Library", "Caches", "navtunnel-cli")
	case "windows":
		dir = os.Getenv("LOCALAPPDATA")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, "AppData", "Local")
		}
		dir = filepath.Join(dir, "NavTunnel")
	default:
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "navtunnel-cli.info"), nil
}

// writeInfo persiste el archivo con permisos 0600.
func writeInfo(info InfoFile) (string, error) {
	path, err := infoPath()
	if err != nil {
		return "", err
	}
	contents := fmt.Sprintf("port=%d\ntoken=%s\npid=%d\n", info.Port, info.Token, info.PID)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ReadInfo devuelve la info del daemon (port + token) si el archivo existe.
// Errores y archivo inexistente deben distinguirse con errors.Is(err, os.ErrNotExist).
func ReadInfo() (InfoFile, string, error) {
	path, err := infoPath()
	if err != nil {
		return InfoFile{}, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return InfoFile{}, path, err
	}
	defer f.Close()

	info := InfoFile{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "port":
			info.Port, _ = strconv.Atoi(val)
		case "token":
			info.Token = val
		case "pid":
			info.PID, _ = strconv.Atoi(val)
		}
	}
	return info, path, nil
}

// removeInfo borra el archivo; ignora "no existe".
func removeInfo() {
	path, err := infoPath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}
