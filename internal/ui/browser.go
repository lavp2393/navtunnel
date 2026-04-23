package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// spawnPanel lanza un subproceso de NavTunnel que abre la ventana nativa
// del panel apuntando a la URL dada. No espera al subproceso: lo reapea en
// una goroutine para evitar zombies.
func spawnPanel(url string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("localizando ejecutable: %w", err)
	}
	cmd := exec.Command(exe, "--panel", url)
	// Heredar el entorno (DISPLAY/WAYLAND_DISPLAY para Linux/X11, y demás).
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openBrowserFallback abre la URL en el navegador por defecto como último
// recurso cuando el subproceso del panel nativo no puede arrancar (p.ej.
// Linux sin libwebkit2gtk).
func openBrowserFallback(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		return nil
	}
	return cmd.Start()
}
