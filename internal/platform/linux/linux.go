package linux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LinuxPlatform implementa las operaciones específicas para Linux.
type LinuxPlatform struct{}

// NewLinux crea una nueva instancia de LinuxPlatform.
func NewLinux() *LinuxPlatform { return &LinuxPlatform{} }

// FindOpenVPN busca el ejecutable de OpenVPN en rutas conocidas y en $PATH.
func (p *LinuxPlatform) FindOpenVPN() (string, error) {
	paths := []string{
		"/usr/sbin/openvpn",
		"/usr/bin/openvpn",
		"/usr/local/sbin/openvpn",
		"/usr/local/bin/openvpn",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	path, err := exec.LookPath("openvpn")
	if err != nil {
		return "", fmt.Errorf("OpenVPN no está instalado. Instala con: sudo apt install openvpn")
	}
	return path, nil
}

// RequiresElevation — openvpn necesita CAP_NET_ADMIN y acceso a /dev/net/tun.
func (p *LinuxPlatform) RequiresElevation() bool { return true }

// ElevateCommand usa pkexec para pedir la autenticación de policykit.
// Requiere un entorno gráfico con policykit-1 instalado (polkit agent).
func (p *LinuxPlatform) ElevateCommand(path string, args []string) (string, []string, error) {
	if _, err := exec.LookPath("pkexec"); err != nil {
		return "", nil, fmt.Errorf("pkexec no disponible. Instala policykit-1: sudo apt install policykit-1")
	}
	elevatedArgs := append([]string{path}, args...)
	return "pkexec", elevatedArgs, nil
}

// GetConfigDir sigue XDG Base Directory.
func (p *LinuxPlatform) GetConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "NavTunnel")
}

// GetDefaultConfigPath sugiere <configDir>/client.ovpn como ruta inicial.
func (p *LinuxPlatform) GetDefaultConfigPath() string {
	if dir := p.GetConfigDir(); dir != "" {
		return filepath.Join(dir, "client.ovpn")
	}
	return ""
}

// GetLogPath sigue XDG para caches/logs.
func (p *LinuxPlatform) GetLogPath() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheHome, "NavTunnel", "logs")
}

func (p *LinuxPlatform) Name() string      { return "linux" }
func (p *LinuxPlatform) Separator() string { return "/" }
