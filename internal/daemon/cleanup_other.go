//go:build !linux

package daemon

// killOrphanOpenVPN en plataformas sin /proc (Windows/macOS) es un no-op.
// En macOS se podría implementar con `ps -A -o pid=,args=` y heurística;
// en Windows con ToolHelp32 snapshot. Por ahora el cleanup corre sólo en
// Linux y en otros OS la limpieza queda a manos del usuario.
func killOrphanOpenVPN() int { return 0 }
