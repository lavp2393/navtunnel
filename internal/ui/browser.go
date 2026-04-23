package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// openAppWindow abre la URL como una ventana dedicada de app — sin tabs, sin
// barra de URL — usando el modo --app de un navegador Chromium. Prueba en
// orden: Chrome, Chromium, Edge, Brave, Vivaldi. Si ninguno está disponible,
// cae al navegador por defecto del sistema (xdg-open / open / start).
func openAppWindow(url string) error {
	if browser := findChromiumBrowser(); browser != "" {
		userData, _ := appBrowserProfileDir()
		args := []string{
			"--app=" + url,
			"--new-window",
			"--window-size=920,720",
		}
		if userData != "" {
			args = append(args, "--user-data-dir="+userData)
		}
		return exec.Command(browser, args...).Start()
	}
	return openDefaultBrowser(url)
}

// findChromiumBrowser busca un navegador basado en Chromium en el orden de
// preferencia. Devuelve el path absoluto o "" si no hay ninguno.
func findChromiumBrowser() string {
	// Primero por PATH (válido en los 3 OS cuando el instalador registra el
	// binario ahí; común en Linux con apt/snap y en Windows con "winget").
	for _, name := range []string{
		"google-chrome-stable", "google-chrome",
		"chromium", "chromium-browser",
		"microsoft-edge-stable", "microsoft-edge",
		"brave-browser", "brave",
		"vivaldi-stable", "vivaldi",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
		}
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pf86 := os.Getenv("ProgramFiles(x86)")
		candidates = []string{
			filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(pf86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(pf, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(pf86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// appBrowserProfileDir devuelve un directorio persistente para el perfil del
// navegador en modo --app. Usar un profile dedicado evita reusar la sesión
// normal del usuario (cookies, extensiones) y garantiza que se abre una
// ventana propia aunque ya tenga el navegador corriendo.
func appBrowserProfileDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "NavTunnel", "app-profile")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// openDefaultBrowser es el fallback cuando no hay Chromium disponible.
func openDefaultBrowser(url string) error {
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
