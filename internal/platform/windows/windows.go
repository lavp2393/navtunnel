package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// WindowsPlatform implementa las operaciones específicas para Windows.
type WindowsPlatform struct{}

// NewWindows crea una nueva instancia de WindowsPlatform.
func NewWindows() *WindowsPlatform { return &WindowsPlatform{} }

// FindOpenVPN busca openvpn.exe en las rutas estándar y en $PATH.
func (p *WindowsPlatform) FindOpenVPN() (string, error) {
	paths := []string{
		`C:\Program Files\OpenVPN\bin\openvpn.exe`,
		`C:\Program Files (x86)\OpenVPN\bin\openvpn.exe`,
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		paths = append(paths, filepath.Join(pf, "OpenVPN", "bin", "openvpn.exe"))
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		paths = append(paths, filepath.Join(pf86, "OpenVPN", "bin", "openvpn.exe"))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	path, err := exec.LookPath("openvpn.exe")
	if err != nil {
		return "", fmt.Errorf("OpenVPN no está instalado. Descarga desde https://openvpn.net/community-downloads/")
	}
	return path, nil
}

// RequiresElevation — openvpn necesita Admin para abrir el adaptador TAP.
func (p *WindowsPlatform) RequiresElevation() bool { return true }

// ElevateCommand en Windows asume que el proceso padre ya corre como Admin.
// El binario se distribuye con un manifest UAC que lo fuerza al arrancar
// (requireAdministrator), por lo que NavTunnel nunca se ejecuta como usuario
// normal. Si por alguna razón no somos admin, retornamos un error claro.
//
// No usamos "runas" porque requiere credenciales; no usamos Start-Process
// -Verb RunAs porque el proceso hijo correría en una sesión separada y
// perderíamos stdout/stderr y el PID.
func (p *WindowsPlatform) ElevateCommand(path string, args []string) (string, []string, error) {
	if !isElevated() {
		return "", nil, fmt.Errorf("NavTunnel requiere ejecutarse como Administrador para controlar el adaptador TAP")
	}
	return path, args, nil
}

// GetConfigDir %APPDATA%\NavTunnel.
func (p *WindowsPlatform) GetConfigDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "NavTunnel")
}

// GetDefaultConfigPath sugiere <configDir>\client.ovpn.
func (p *WindowsPlatform) GetDefaultConfigPath() string {
	if dir := p.GetConfigDir(); dir != "" {
		return filepath.Join(dir, "client.ovpn")
	}
	return ""
}

// GetLogPath %LOCALAPPDATA%\NavTunnel\logs.
func (p *WindowsPlatform) GetLogPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, "NavTunnel", "logs")
}

func (p *WindowsPlatform) Name() string      { return "windows" }
func (p *WindowsPlatform) Separator() string { return `\` }
