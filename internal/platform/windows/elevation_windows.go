//go:build windows

package windows

import winapi "golang.org/x/sys/windows"

// isElevated devuelve true si el proceso corre con privilegios de Administrador.
// Usa el token del proceso actual y chequea el flag TokenElevation.
func isElevated() bool {
	var token winapi.Token
	err := winapi.OpenProcessToken(winapi.CurrentProcess(), winapi.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}
