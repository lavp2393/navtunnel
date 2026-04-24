package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Signature que NavTunnel usa al invocar openvpn:
//
//	openvpn --management ... /tmp/navtunnel-mgmt-*.pw --management-query-passwords ...
//
// Cualquier otro openvpn (manual con sudo, NetworkManager, Tunnelblick,
// openvpn3, etc.) NO usa `--management-hold` acompañado del pw-file
// "navtunnel-mgmt-*". Buscamos la coincidencia estricta de ese flag y de
// ese basename en el pw-file para no tocar procesos ajenos.
const (
	ovpnSignatureFlag   = "--management-hold"
	ovpnSignatureFilePfx = "navtunnel-mgmt-"
)

// killOrphanOpenVPN busca procesos openvpn que lleven nuestras firmas
// (typical de un daemon previo que murió abruptamente sin desconectar) y
// les manda SIGTERM. Retorna la cantidad de procesos a los que mandó señal.
//
// Lee /proc/*/cmdline en Linux (en macOS/Windows por ahora no hace nada).
// Usa SIGTERM, no SIGKILL: si openvpn está vivo y sano, sale limpio
// liberando el tun device y las rutas que haya agregado.
func killOrphanOpenVPN() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		// /proc/<pid>/cmdline separa argumentos con NUL bytes.
		args := strings.Split(string(data), "\x00")
		if !looksLikeOurOpenVPN(args) {
			continue
		}
		// SIGTERM: openvpn hace shutdown limpio (remove routes, tun down).
		if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
			killed++
		}
	}
	if killed > 0 {
		// Darle un segundo para que baje el tun device. No forzamos KILL:
		// si el proceso sigue vivo después, el usuario ve el error en el
		// log del daemon. No queremos arriesgar matar algo que está en
		// medio de un shutdown legítimo.
		time.Sleep(time.Second)
	}
	return killed
}

// looksLikeOurOpenVPN chequea que el cmdline tenga nuestra firma: binario
// openvpn + flag --management-hold + pw-file con prefijo navtunnel-mgmt-.
// Todos los tres deben estar presentes; un openvpn vecino (manual o via
// NetworkManager) no coincide.
func looksLikeOurOpenVPN(args []string) bool {
	if len(args) == 0 {
		return false
	}
	base := filepath.Base(args[0])
	if base != "openvpn" && base != "openvpn.exe" {
		return false
	}
	hasHold := false
	hasOurPWFile := false
	for _, a := range args {
		if a == ovpnSignatureFlag {
			hasHold = true
		}
		if strings.Contains(filepath.Base(a), ovpnSignatureFilePfx) {
			hasOurPWFile = true
		}
	}
	return hasHold && hasOurPWFile
}
