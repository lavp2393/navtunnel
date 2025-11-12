package tray

import (
	"sync"

	"github.com/getlantern/systray"
)

// Systray es la implementación usando getlantern/systray
type Systray struct {
	callbacks MenuCallbacks
	mu        sync.RWMutex

	// Menu items
	mStatus      *systray.MenuItem
	mConnect     *systray.MenuItem
	mDisconnect  *systray.MenuItem
	mShowWindow  *systray.MenuItem
	mQuit        *systray.MenuItem

	currentState string
	currentIcon  IconType
}

// NewSystray crea una nueva instancia de Systray
func NewSystray(callbacks MenuCallbacks) *Systray {
	return &Systray{
		callbacks:    callbacks,
		currentState: "Desconectado",
		currentIcon:  IconDisconnected,
	}
}

// Run inicia el system tray (blocking)
func (s *Systray) Run(onReady func(), onExit func()) {
	systray.Run(func() {
		s.onReady()
		if onReady != nil {
			onReady()
		}
	}, func() {
		if onExit != nil {
			onExit()
		}
	})
}

// onReady se llama cuando systray está listo
func (s *Systray) onReady() {
	// Configurar icono y título inicial
	systray.SetIcon(GetIconData(IconDisconnected))
	systray.SetTitle("PreyVPN")
	systray.SetTooltip("PreyVPN - Desconectado")

	// Crear menú
	s.mStatus = systray.AddMenuItem("Estado: Desconectado", "Estado actual de la conexión")
	s.mStatus.Disable()

	systray.AddSeparator()

	s.mConnect = systray.AddMenuItem("Conectar", "Conectar a la VPN")
	s.mDisconnect = systray.AddMenuItem("Desconectar", "Desconectar de la VPN")
	s.mDisconnect.Disable() // Inicialmente deshabilitado

	systray.AddSeparator()

	s.mShowWindow = systray.AddMenuItem("Abrir ventana", "Mostrar ventana principal")

	systray.AddSeparator()

	s.mQuit = systray.AddMenuItem("Salir", "Cerrar PreyVPN")

	// Iniciar goroutines para manejar clics
	go s.handleMenuActions()
}

// handleMenuActions maneja los clics en el menú
func (s *Systray) handleMenuActions() {
	for {
		select {
		case <-s.mConnect.ClickedCh:
			if s.callbacks.OnConnect != nil {
				s.callbacks.OnConnect()
			}

		case <-s.mDisconnect.ClickedCh:
			if s.callbacks.OnDisconnect != nil {
				s.callbacks.OnDisconnect()
			}

		case <-s.mShowWindow.ClickedCh:
			if s.callbacks.OnShowWindow != nil {
				s.callbacks.OnShowWindow()
			}

		case <-s.mQuit.ClickedCh:
			if s.callbacks.OnQuit != nil {
				s.callbacks.OnQuit()
			}
			systray.Quit()
			return
		}
	}
}

// SetTitle actualiza el título del tray icon
func (s *Systray) SetTitle(title string) {
	systray.SetTitle(title)
}

// SetIcon actualiza el icono según el estado
func (s *Systray) SetIcon(iconType IconType) {
	s.mu.Lock()
	s.currentIcon = iconType
	s.mu.Unlock()

	// Solo actualizar si hay datos del icono
	iconData := GetIconData(iconType)
	if len(iconData) > 0 {
		systray.SetIcon(iconData)
	}
}

// SetTooltip actualiza el tooltip
func (s *Systray) SetTooltip(text string) {
	systray.SetTooltip(text)
}

// UpdateState actualiza el estado visible en el menú
func (s *Systray) UpdateState(state string, connected bool) {
	s.mu.Lock()
	s.currentState = state
	s.mu.Unlock()

	// Actualizar texto del menú de estado
	if s.mStatus != nil {
		s.mStatus.SetTitle("Estado: " + state)
	}

	// Actualizar tooltip
	s.SetTooltip("PreyVPN - " + state)

	// Habilitar/deshabilitar botones según estado
	// Solo si los items del menú están inicializados
	if s.mConnect != nil && s.mDisconnect != nil {
		if connected {
			s.mConnect.Disable()
			s.mDisconnect.Enable()
		} else {
			s.mConnect.Enable()
			s.mDisconnect.Disable()
		}
	}
}

// Quit cierra el tray icon
func (s *Systray) Quit() {
	systray.Quit()
}
