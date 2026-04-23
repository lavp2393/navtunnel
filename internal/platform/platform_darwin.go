//go:build darwin

package platform

import "github.com/lavp2393/navtunnel/internal/platform/darwin"

// darwinAdapter expone darwin.DarwinPlatform como Platform.
type darwinAdapter struct {
	impl *darwin.DarwinPlatform
}

// NewDarwin retorna la implementación de Platform para macOS.
func NewDarwin() Platform {
	return &darwinAdapter{impl: darwin.NewDarwin()}
}

func (a *darwinAdapter) FindOpenVPN() (string, error) { return a.impl.FindOpenVPN() }
func (a *darwinAdapter) RequiresElevation() bool      { return a.impl.RequiresElevation() }
func (a *darwinAdapter) ElevateCommand(path string, args []string) (string, []string, error) {
	return a.impl.ElevateCommand(path, args)
}
func (a *darwinAdapter) GetConfigDir() string         { return a.impl.GetConfigDir() }
func (a *darwinAdapter) GetDefaultConfigPath() string { return a.impl.GetDefaultConfigPath() }
func (a *darwinAdapter) GetLogPath() string           { return a.impl.GetLogPath() }
func (a *darwinAdapter) Name() string                 { return a.impl.Name() }
func (a *darwinAdapter) Separator() string            { return a.impl.Separator() }

func NewLinux() Platform   { panic("Linux platform not available on Darwin builds") }
func NewWindows() Platform { panic("Windows platform not available on Darwin builds") }
