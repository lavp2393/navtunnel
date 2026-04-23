//go:build windows

package platform

import "github.com/lavp2393/navtunnel/internal/platform/windows"

// windowsAdapter expone windows.WindowsPlatform como Platform.
type windowsAdapter struct {
	impl *windows.WindowsPlatform
}

// NewWindows retorna la implementación de Platform para Windows.
func NewWindows() Platform {
	return &windowsAdapter{impl: windows.NewWindows()}
}

func (a *windowsAdapter) FindOpenVPN() (string, error) { return a.impl.FindOpenVPN() }
func (a *windowsAdapter) RequiresElevation() bool      { return a.impl.RequiresElevation() }
func (a *windowsAdapter) ElevateCommand(path string, args []string) (string, []string, error) {
	return a.impl.ElevateCommand(path, args)
}
func (a *windowsAdapter) GetConfigDir() string         { return a.impl.GetConfigDir() }
func (a *windowsAdapter) GetDefaultConfigPath() string { return a.impl.GetDefaultConfigPath() }
func (a *windowsAdapter) GetLogPath() string           { return a.impl.GetLogPath() }
func (a *windowsAdapter) Name() string                 { return a.impl.Name() }
func (a *windowsAdapter) Separator() string            { return a.impl.Separator() }

func NewLinux() Platform  { panic("Linux platform not available on Windows builds") }
func NewDarwin() Platform { panic("Darwin platform not available on Windows builds") }
