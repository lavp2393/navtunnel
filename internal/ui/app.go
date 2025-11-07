package ui

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prey/preyvpn/internal/core"
	"github.com/prey/preyvpn/internal/logs"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// AppState representa el estado de la aplicación
type AppState int

const (
	StateDisconnected AppState = iota
	StateConnecting
	StateAuthenticating
	StateConnected
	StateError
)

// App representa la aplicación principal
type App struct {
	fyneApp   fyne.App
	window    fyne.Window
	state     AppState
	logBuffer *logs.Buffer

	// Thread-safety for state
	stateMutex sync.RWMutex

	// UI elements
	statusLabel   *widget.Label
	connectBtn    *widget.Button
	disconnectBtn *widget.Button
	retryBtn      *widget.Button
	logView       *widget.Entry
	configStatus  *widget.Label

	// Core components
	manager *core.Manager
	sendFns core.SendFns

	// Credentials cache (in-memory for current session)
	savedUsername string
	savedPassword string
	rememberCreds bool
	credStore     core.CredentialStoreMethod
}

// NewApp crea una nueva instancia de la aplicación
func NewApp() *App {
	a := &App{
		fyneApp:   app.New(),
		logBuffer: logs.NewBuffer(30),
		state:     StateDisconnected,
	}

	a.window = a.fyneApp.NewWindow("PreyVPN")
	a.window.Resize(fyne.NewSize(700, 500))
	a.buildUI()
	a.initializeStoredCredentials()

	return a
}

// buildUI construye la interfaz de usuario
func (a *App) buildUI() {
	// Status label
	a.statusLabel = widget.NewLabel("Estado: Desconectado")
	a.statusLabel.Wrapping = fyne.TextWrapWord

	// Config status
	a.configStatus = widget.NewLabel("")

	// Buttons
	a.connectBtn = widget.NewButton("Conectar", a.onConnect)
	a.disconnectBtn = widget.NewButton("Desconectar", a.onDisconnect)
	a.disconnectBtn.Disable()

	a.retryBtn = widget.NewButton("Reintentar", func() {
		a.updateConfigStatus()
		if core.CheckConfigExists() {
			a.connectBtn.Enable()
			a.retryBtn.Hide()
		}
	})
	a.retryBtn.Hide()

	// Actualizar estado del config después de crear todos los widgets
	a.updateConfigStatus()

	// Log view (read-only)
	a.logView = widget.NewMultiLineEntry()
	a.logView.Disable() // Read-only
	a.logView.SetPlaceHolder("Los logs aparecerán aquí...")

	// Layout
	buttonBox := container.NewHBox(
		a.connectBtn,
		a.disconnectBtn,
		a.retryBtn,
	)

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("PreyVPN - Cliente OpenVPN"),
			widget.NewSeparator(),
			a.configStatus,
			a.statusLabel,
			buttonBox,
			widget.NewSeparator(),
			widget.NewLabel("Logs:"),
		),
		nil,
		nil,
		nil,
		container.NewScroll(a.logView),
	)

	a.window.SetContent(content)
}

// initializeStoredCredentials intenta recuperar credenciales guardadas y actualiza el estado interno
func (a *App) initializeStoredCredentials() {
	username, password, method, warning, err := core.LoadCredentials()
	if err == nil {
		a.savedUsername = username
		a.savedPassword = password
		a.rememberCreds = true
		a.credStore = method
		a.logCredentialLoad(method)
		if warning != "" {
			a.addLog("Aviso: " + warning)
		}
		return
	}

	if warning != "" {
		a.addLog("Aviso: " + warning)
	}

	if errors.Is(err, core.ErrCredentialsNotFound) {
		return
	}

	a.addLog("Advertencia: No se pudieron cargar credenciales guardadas: " + err.Error())
}

// updateConfigStatus actualiza el estado del archivo de configuración
func (a *App) updateConfigStatus() {
	if core.CheckConfigExists() {
		a.configStatus.SetText("✅ Perfil detectado: ~/PreyVPN/prey-prod.ovpn")
		a.connectBtn.Enable()
		a.retryBtn.Hide()
	} else {
		a.configStatus.SetText("❌ Perfil no encontrado\n\nPor favor coloca tu archivo prey-prod.ovpn en ~/PreyVPN/")
		a.connectBtn.Disable()
		a.retryBtn.Show()
	}
}

// onConnect maneja el evento de conectar
func (a *App) onConnect() {
	// Verificar que exista el config
	if !core.CheckConfigExists() {
		ShowError(a.window, "Error", "No se encontró el archivo de configuración en ~/PreyVPN/prey-prod.ovpn")
		return
	}

	a.addLog("Iniciando conexión VPN...")

	// Obtener la ruta del config
	configPath := core.GetConfigPath()

	// Buscar el binario de OpenVPN
	openvpnPath, err := core.FindOpenVPN()
	if err != nil {
		a.addLog("Error al buscar OpenVPN: " + err.Error())
		ShowError(a.window, "Error", "No se encontró OpenVPN. ¿Está instalado?")
		return
	}

	a.addLog(fmt.Sprintf("Usando OpenVPN: %s", openvpnPath))

	// Iniciar el manager directamente (sin Management Interface)
	// El manager se encarga de lanzar OpenVPN con pipes directos
	mgr, err := core.Start(configPath, openvpnPath)
	if err != nil {
		a.addLog("Error al iniciar OpenVPN: " + err.Error())
		ShowError(a.window, "Error", err.Error())
		return
	}

	a.manager = mgr
	a.sendFns = mgr.SendFunctions()

	// Actualizar UI
	a.setState(StateConnecting)
	a.connectBtn.Disable()
	a.disconnectBtn.Enable()

	a.addLog("Esperando prompts de autenticación...")

	// Iniciar procesamiento de eventos
	go a.handleEvents()
}

// onDisconnect maneja el evento de desconectar
func (a *App) onDisconnect() {
	a.addLog("Desconectando...")
	// Set state immediately to prevent race conditions in callbacks
	a.setState(StateDisconnected)

	// El manager se encarga de matar el proceso OpenVPN cuando se llama Stop()
	if a.manager != nil {
		a.manager.Stop()
		a.addLog("Proceso OpenVPN detenido")
		a.manager = nil
	}

	a.connectBtn.Enable()
	a.disconnectBtn.Disable()
}

// handleEvents procesa los eventos del manager
func (a *App) handleEvents() {
	for event := range a.manager.Events() {
		switch event.Type {
		case core.EventLogLine:
			a.addLog(event.Message)

		case core.EventAskUser:
			a.setState(StateAuthenticating)
			ShowUsernamePromptWithRemember(a.window, a.savedUsername, a.rememberCreds, func(result PromptResult) {
				if a.getState() != StateAuthenticating {
					return // Abort if state changed (e.g., disconnected)
				}
				a.savedUsername = result.Value
				a.rememberCreds = result.Remember

				// Enviar username a OpenVPN
				if err := a.sendFns.Username(result.Value); err != nil {
					a.addLog("Error al enviar usuario: " + err.Error())
				}
			})

		case core.EventAskPass:
			a.setState(StateAuthenticating)
			ShowPasswordPromptWithDefault(a.window, a.savedPassword, func(password string) {
				if a.getState() != StateAuthenticating {
					return // Abort if state changed
				}
				a.savedPassword = password

				if a.rememberCreds {
					method, warning, err := core.SaveCredentials(a.savedUsername, a.savedPassword)
					if err != nil {
						a.addLog("Advertencia: No se pudieron guardar las credenciales: " + err.Error())
					} else {
						a.credStore = method
						a.logCredentialSave(method)
						if warning != "" {
							a.addLog("Aviso: " + warning)
						}
					}
				} else {
					if err := core.DeleteCredentials(); err != nil {
						a.addLog("Advertencia: No se pudieron eliminar credenciales guardadas: " + err.Error())
					}
					a.savedUsername = ""
					a.savedPassword = ""
					a.credStore = core.CredentialStoreMethodNone
				}

				if err := a.sendFns.Password(password); err != nil {
					a.addLog("Error al enviar contraseña: " + err.Error())
				}
			})

		case core.EventAskOTP:
			a.setState(StateAuthenticating)
			ShowOTPPrompt(a.window, func(otp string) {
				if a.getState() != StateAuthenticating {
					return // Abort if state changed
				}
				if err := a.sendFns.OTP(otp); err != nil {
					a.addLog("Error al enviar OTP: " + err.Error())
				}
			})

		case core.EventConnected:
			a.setState(StateConnected)
			a.addLog(event.Message)
			ShowInfo(a.window, "Conectado", "Conexión VPN establecida exitosamente")

		case core.EventAuthFailed:
			a.setState(StateAuthenticating)
			a.addLog("Error: " + event.Message)
			ShowError(a.window, "Error de autenticación", event.Message)

			if event.Stage == "password" {
				ShowPasswordPromptWithDefault(a.window, a.savedPassword, func(password string) {
					if a.getState() != StateAuthenticating {
						return // Abort if state changed
					}
					a.savedPassword = password
					if a.rememberCreds {
						// Re-save credentials on failure if remember is checked
						if _, _, err := core.SaveCredentials(a.savedUsername, a.savedPassword); err != nil {
							a.addLog("Advertencia: No se pudieron actualizar las credenciales: " + err.Error())
						}
					}
					if err := a.sendFns.Password(password); err != nil {
						a.addLog("Error al enviar contraseña: " + err.Error())
					}
				})
			} else if event.Stage == "otp" {
				ShowOTPPrompt(a.window, func(otp string) {
					if a.getState() != StateAuthenticating {
						return // Abort if state changed
					}
					if err := a.sendFns.OTP(otp); err != nil {
						a.addLog("Error al enviar OTP: " + err.Error())
					}
				})
			}

		case core.EventFatal:
			a.setState(StateError)
			a.addLog("Error fatal: " + event.Message)
			ShowError(a.window, "Error Fatal", event.Message)
			a.onDisconnect()

		case core.EventDisconnected:
			a.addLog("Conexión cerrada")
			a.onDisconnect()
		}
	}
}

// getState de forma segura para hilos
func (a *App) getState() AppState {
	a.stateMutex.RLock()
	defer a.stateMutex.RUnlock()
	return a.state
}

// setState actualiza el estado de la aplicación de forma segura para hilos
func (a *App) setState(state AppState) {
	a.stateMutex.Lock()
	a.state = state
	a.stateMutex.Unlock()

	switch state {
	case StateDisconnected:
		a.statusLabel.SetText("Estado: Desconectado")
	case StateConnecting:
		a.statusLabel.SetText("Estado: Conectando...")
	case StateAuthenticating:
		a.statusLabel.SetText("Estado: Autenticando...")
	case StateConnected:
		a.statusLabel.SetText("Estado: Conectado ✅")
	case StateError:
		a.statusLabel.SetText("Estado: Error ❌")
	}
	a.statusLabel.Refresh()
}

// addLog agrega una línea al buffer de logs y actualiza la UI
func (a *App) addLog(line string) {
	a.logBuffer.Add(line)
	a.logView.SetText(a.logBuffer.GetText())

	// Auto-scroll al final
	if a.logView.Visible() {
		a.logView.CursorRow = len(a.logBuffer.GetAll())
	}
	a.logView.Refresh()
}

func (a *App) logCredentialSave(method core.CredentialStoreMethod) {
	switch method {
	case core.CredentialStoreMethodKeyring:
		a.addLog("✓ Credenciales guardadas en el keyring del sistema")
	case core.CredentialStoreMethodFile:
		a.addLog("✓ Credenciales guardadas en archivo seguro: " + fallbackCredentialsPathLabel())
	}
}

func (a *App) logCredentialLoad(method core.CredentialStoreMethod) {
	switch method {
	case core.CredentialStoreMethodKeyring:
		a.addLog("✓ Credenciales cargadas desde el keyring del sistema")
	case core.CredentialStoreMethodFile:
		a.addLog("✓ Credenciales cargadas desde archivo local: " + fallbackCredentialsPathLabel())
	}
}

func fallbackCredentialsPathLabel() string {
	if path := core.GetCredentialsFallbackPath(); path != "" {
		return path
	}
	return "~/.config/PreyVPN/credentials.json"
}

// Run inicia la aplicación
func (a *App) Run() {
	a.window.ShowAndRun()
}
