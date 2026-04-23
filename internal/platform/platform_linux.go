//go:build linux

package platform

import "github.com/lavp2393/navtunnel/internal/platform/linux"

// linuxAdapter expone linux.LinuxPlatform como Platform.
type linuxAdapter struct {
	impl *linux.LinuxPlatform
}

// NewLinux retorna la implementación de Platform para Linux.
func NewLinux() Platform {
	return &linuxAdapter{impl: linux.NewLinux()}
}

func (a *linuxAdapter) FindOpenVPN() (string, error) { return a.impl.FindOpenVPN() }
func (a *linuxAdapter) RequiresElevation() bool      { return a.impl.RequiresElevation() }
func (a *linuxAdapter) ElevateCommand(path string, args []string) (string, []string, error) {
	return a.impl.ElevateCommand(path, args)
}
func (a *linuxAdapter) GetConfigDir() string         { return a.impl.GetConfigDir() }
func (a *linuxAdapter) GetDefaultConfigPath() string { return a.impl.GetDefaultConfigPath() }
func (a *linuxAdapter) GetLogPath() string           { return a.impl.GetLogPath() }
func (a *linuxAdapter) Name() string                 { return a.impl.Name() }
func (a *linuxAdapter) Separator() string            { return a.impl.Separator() }

// En una build de Linux, las otras plataformas no están disponibles y deben
// provocar un pánico claro si alguien las instancia por error.
func NewWindows() Platform { panic("Windows platform not available on Linux builds") }
func NewDarwin() Platform  { panic("Darwin platform not available on Linux builds") }
