//go:build !windows

package windows

// isElevated en no-Windows es un stub: el código que lo llama solo se ejecuta
// bajo GOOS=windows. Devuelve true para que compilaciones cruzadas no revienten.
func isElevated() bool { return true }
