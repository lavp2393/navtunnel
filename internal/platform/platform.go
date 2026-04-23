package platform

import "runtime"

// Platform abstrae las operaciones específicas de cada sistema operativo.
// El Manager no arranca openvpn directamente por aquí: solicita los argumentos
// de elevación vía ElevateCommand y gobierna el proceso él mismo para controlar
// el management socket.
type Platform interface {
	// FindOpenVPN retorna la ruta absoluta al binario openvpn.
	FindOpenVPN() (string, error)

	// RequiresElevation indica si openvpn necesita privilegios de root/admin.
	RequiresElevation() bool

	// ElevateCommand traduce (path, args) al par (program, args) equivalente
	// ejecutado con privilegios. En Linux es pkexec; en macOS osascript;
	// en Windows se asume que el proceso padre ya es Admin y devuelve
	// (path, args) sin cambios, o error si no lo es.
	ElevateCommand(path string, args []string) (string, []string, error)

	// GetConfigDir es el directorio donde guardar config de usuario (XDG en
	// Linux, Application Support en macOS, APPDATA en Windows).
	GetConfigDir() string

	// GetDefaultConfigPath es una sugerencia inicial para el archivo .ovpn.
	// El usuario puede sobrescribirla desde la UI.
	GetDefaultConfigPath() string

	// GetLogPath es el directorio recomendado para logs persistentes.
	GetLogPath() string

	// Name del OS: "linux" | "darwin" | "windows".
	Name() string

	// Separator de rutas ("/" o "\\").
	Separator() string
}

// New retorna la implementación activa según runtime.GOOS.
func New() Platform {
	switch runtime.GOOS {
	case "linux":
		return NewLinux()
	case "windows":
		return NewWindows()
	case "darwin":
		return NewDarwin()
	default:
		panic("unsupported platform: " + runtime.GOOS)
	}
}
