package ui

import (
	"os/exec"
	"runtime"
)

// openBrowser abre la URL en el navegador por defecto del usuario.
// Es best-effort; si el comando falla, el error se propaga para loguearlo.
func openBrowser(url string) error {
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
