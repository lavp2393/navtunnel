package tray

// TrayIcon representa la abstracción del icono de system tray
// Esta interfaz permite cambiar la implementación (systray, AppIndicator3, etc.)
// sin afectar el resto del código
type TrayIcon interface {
	// SetTitle actualiza el título del icono (tooltip)
	SetTitle(title string)

	// SetIcon actualiza el icono (según el estado)
	SetIcon(iconType IconType)

	// SetTooltip actualiza el texto del tooltip
	SetTooltip(text string)

	// Run inicia el tray icon (blocking)
	Run(onReady func(), onExit func())

	// Quit cierra el tray icon
	Quit()
}

// IconType representa los diferentes estados del icono
type IconType int

const (
	IconDisconnected IconType = iota
	IconConnecting
	IconConnected
	IconError
)

// MenuCallbacks contiene los callbacks para las acciones del menú
type MenuCallbacks struct {
	OnConnect    func()
	OnDisconnect func()
	OnShowWindow func()
	OnQuit       func()
}

// New crea una nueva instancia de TrayIcon
// Por defecto usa systray, pero puede cambiarse a AppIndicator3 en el futuro
func New(callbacks MenuCallbacks) TrayIcon {
	return NewSystray(callbacks)
}
