package ui

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// errNoPicker indica que no hay diálogo nativo disponible en esta plataforma
// (p.ej. Linux sin zenity ni kdialog). El server responde 501 y la UI cae
// al input de texto manual.
var errNoPicker = errors.New("no hay diálogo nativo disponible")

// pickOvpnFile abre un diálogo nativo para elegir un archivo .ovpn y devuelve
// su ruta absoluta. Retorna errNoPicker si la plataforma no tiene un binario
// apropiado instalado.
func pickOvpnFile() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return pickLinux()
	case "darwin":
		return pickDarwin()
	case "windows":
		return pickWindows()
	default:
		return "", errNoPicker
	}
}

func pickLinux() (string, error) {
	if path, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command(path,
			"--file-selection",
			"--title=Selecciona archivo .ovpn",
			"--file-filter=Archivos OpenVPN | *.ovpn",
		).Output()
		if err != nil {
			// zenity devuelve código 1 si el usuario cancela.
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command(path,
			"--getopenfilename",
			".",
			"*.ovpn|Archivos OpenVPN",
		).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", errNoPicker
}

func pickDarwin() (string, error) {
	script := `choose file of type {"ovpn"} with prompt "Selecciona archivo .ovpn"`
	out, err := exec.Command("osascript", "-e",
		"set f to POSIX path of ("+script+")",
		"-e", "return f",
	).Output()
	if err != nil {
		// El usuario canceló — osascript devuelve exit 1.
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func pickWindows() (string, error) {
	ps := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$d = New-Object System.Windows.Forms.OpenFileDialog; ` +
		`$d.Filter = 'OpenVPN (*.ovpn)|*.ovpn'; ` +
		`$null = $d.ShowDialog(); ` +
		`$d.FileName`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return "", errNoPicker
	}
	return strings.TrimSpace(string(out)), nil
}
