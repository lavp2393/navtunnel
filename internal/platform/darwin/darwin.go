package darwin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DarwinPlatform implementa las operaciones específicas para macOS.
type DarwinPlatform struct{}

// NewDarwin crea una nueva instancia de DarwinPlatform.
func NewDarwin() *DarwinPlatform { return &DarwinPlatform{} }

// FindOpenVPN busca openvpn en las rutas típicas de Homebrew y en $PATH.
func (p *DarwinPlatform) FindOpenVPN() (string, error) {
	paths := []string{
		"/opt/homebrew/sbin/openvpn",                             // Apple Silicon Homebrew
		"/opt/homebrew/opt/openvpn/sbin/openvpn",                 // Apple Silicon Homebrew (cellar)
		"/usr/local/sbin/openvpn",                                // Intel Homebrew
		"/usr/local/opt/openvpn/sbin/openvpn",                    // Intel Homebrew (cellar)
		"/usr/local/bin/openvpn",                                 // Manual/otros
		"/Applications/Tunnelblick.app/Contents/Resources/openvpn",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	path, err := exec.LookPath("openvpn")
	if err != nil {
		return "", fmt.Errorf("OpenVPN no está instalado. Instala con: brew install openvpn")
	}
	return path, nil
}

// RequiresElevation — openvpn necesita root para configurar utun y rutas.
func (p *DarwinPlatform) RequiresElevation() bool { return true }

// ElevateCommand envuelve el binario en un AppleScript que lo ejecuta con
// "administrator privileges": macOS muestra el diálogo nativo de autorización
// y luego corre openvpn como root. osascript queda como proceso padre y
// bloquea hasta que openvpn termine, así podemos esperarlo con cmd.Wait().
//
// NOTA: al matar el proceso padre (osascript), macOS no propaga la señal al
// openvpn elevado. El Manager hace shutdown limpio vía management socket
// ("signal SIGTERM") antes de caer al Kill del padre.
func (p *DarwinPlatform) ElevateCommand(path string, args []string) (string, []string, error) {
	parts := []string{shellQuote(path)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	shell := strings.Join(parts, " ")

	script := fmt.Sprintf(`do shell script %s with administrator privileges`, appleScriptQuote(shell))
	return "osascript", []string{"-e", script}, nil
}

// shellQuote cita un argumento con single-quotes de POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// appleScriptQuote cita una string para AppleScript (comillas dobles y escapes).
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// GetConfigDir ~/Library/Application Support/NavTunnel.
func (p *DarwinPlatform) GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "NavTunnel")
}

// GetDefaultConfigPath sugiere <configDir>/client.ovpn.
func (p *DarwinPlatform) GetDefaultConfigPath() string {
	if dir := p.GetConfigDir(); dir != "" {
		return filepath.Join(dir, "client.ovpn")
	}
	return ""
}

// GetLogPath ~/Library/Logs/NavTunnel.
func (p *DarwinPlatform) GetLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Logs", "NavTunnel")
}

func (p *DarwinPlatform) Name() string      { return "darwin" }
func (p *DarwinPlatform) Separator() string { return "/" }
