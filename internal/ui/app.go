package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/prey/preyvpn/internal/config"
	"github.com/prey/preyvpn/internal/core"
	"github.com/prey/preyvpn/internal/logs"
	"github.com/prey/preyvpn/internal/tray"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
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
	trayIcon  *tray.Systray
	config    *config.Config

	// Thread-safety for state
	stateMutex sync.RWMutex

	// UI elements
	statusLabel   *widget.Label
	connectBtn    *widget.Button
	disconnectBtn *widget.Button
	retryBtn      *widget.Button
	changeFileBtn *widget.Button
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

	// Cargar o crear configuración
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			// Primera ejecución - crear configuración por defecto
			cfg = config.Default()
		} else {
			// Error cargando configuración
			cfg = config.Default()
		}
	}
	a.config = cfg

	a.window = a.fyneApp.NewWindow("PreyVPN")
	a.window.Resize(fyne.NewSize(700, 500))

	// Configurar comportamiento al cerrar: minimizar a tray en vez de salir
	a.window.SetCloseIntercept(func() {
		a.window.Hide()
	})

	a.buildUI()
	a.initializeStoredCredentials()
	a.setupTrayIcon()

	// Si no hay archivo .ovpn configurado, mostrar file picker
	if !a.config.HasVPNConfig() || !a.config.IsVPNConfigValid() {
		a.showWelcomeDialog()
	}

	return a
}

// setupTrayIcon configura el icono de system tray
func (a *App) setupTrayIcon() {
	callbacks := tray.MenuCallbacks{
		OnConnect: func() {
			a.window.Show() // Mostrar ventana primero
			a.onConnect()
		},
		OnDisconnect: func() {
			a.onDisconnect()
		},
		OnShowWindow: func() {
			a.window.Show()
			a.window.RequestFocus()
		},
		OnQuit: func() {
			// Desconectar si está conectado
			if a.manager != nil {
				a.manager.Stop()
			}
			a.fyneApp.Quit()
		},
	}

	a.trayIcon = tray.NewSystray(callbacks)
	// NO llamar updateTrayIcon() aquí - el tray aún no está inicializado
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
		if a.config.IsVPNConfigValid() {
			a.connectBtn.Enable()
			a.retryBtn.Hide()
		}
	})
	a.retryBtn.Hide()

	a.changeFileBtn = widget.NewButton("Cambiar archivo VPN", func() {
		a.showFilePicker()
	})

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
		a.changeFileBtn,
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
	if a.config.IsVPNConfigValid() {
		fileName := filepath.Base(a.config.VPNConfigPath)
		a.configStatus.SetText(fmt.Sprintf("✅ Archivo VPN: %s", fileName))
		a.connectBtn.Enable()
		a.retryBtn.Hide()
		a.changeFileBtn.Show()
	} else if a.config.HasVPNConfig() {
		// Tiene configurado pero el archivo no existe
		fileName := filepath.Base(a.config.VPNConfigPath)
		a.configStatus.SetText(fmt.Sprintf("❌ Archivo no encontrado: %s", fileName))
		a.connectBtn.Disable()
		a.retryBtn.Hide()
		a.changeFileBtn.Show()
	} else {
		// No hay archivo configurado
		a.configStatus.SetText("⚠️  No hay archivo VPN configurado")
		a.connectBtn.Disable()
		a.retryBtn.Hide()
		a.changeFileBtn.Show()
	}
}

// onConnect maneja el evento de conectar
func (a *App) onConnect() {
	// Verificar que exista el config
	if !a.config.IsVPNConfigValid() {
		ShowError(a.window, "Error", "No se encontró el archivo de configuración VPN. Por favor selecciona un archivo.")
		a.showFilePicker()
		return
	}

	a.addLog("Iniciando conexión VPN...")

	// Obtener la ruta del config
	configPath := a.config.VPNConfigPath

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

	// Actualizar tray icon también
	a.updateTrayIcon()
}

// updateTrayIcon actualiza el icono y estado del system tray
func (a *App) updateTrayIcon() {
	if a.trayIcon == nil {
		return
	}

	state := a.getState()
	switch state {
	case StateDisconnected:
		a.trayIcon.SetIcon(tray.IconDisconnected)
		a.trayIcon.UpdateState("Desconectado", false)

	case StateConnecting:
		a.trayIcon.SetIcon(tray.IconConnecting)
		a.trayIcon.UpdateState("Conectando...", false)

	case StateAuthenticating:
		a.trayIcon.SetIcon(tray.IconConnecting)
		a.trayIcon.UpdateState("Autenticando...", false)

	case StateConnected:
		a.trayIcon.SetIcon(tray.IconConnected)
		a.trayIcon.UpdateState("Conectado", true)

	case StateError:
		a.trayIcon.SetIcon(tray.IconError)
		a.trayIcon.UpdateState("Error", false)
	}
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

// showWelcomeDialog muestra el diálogo de bienvenida para primera ejecución
func (a *App) showWelcomeDialog() {
	dialog.ShowCustom(
		"👋 Bienvenido a PreyVPN",
		"Continuar",
		widget.NewLabel("Para comenzar, necesitas seleccionar tu archivo de configuración VPN (.ovpn)"),
		a.window,
	)

	// Dar tiempo para que el usuario lea el mensaje
	go func() {
		// Esperar un poco y luego mostrar el file picker
		a.showFilePicker()
	}()
}

// showFilePicker muestra el selector de archivos para elegir un .ovpn
func (a *App) showFilePicker() {
	// Crear file dialog
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			a.addLog("Error al seleccionar archivo: " + err.Error())
			return
		}
		if reader == nil {
			// Usuario canceló
			a.addLog("Selección de archivo cancelada")
			return
		}

		// Obtener la ruta del archivo
		filePath := reader.URI().Path()
		reader.Close()

		// Verificar que sea un archivo .ovpn
		if filepath.Ext(filePath) != ".ovpn" {
			ShowError(a.window, "Error", "Por favor selecciona un archivo .ovpn válido")
			return
		}

		// Guardar en configuración
		a.config.VPNConfigPath = filePath
		if err := a.config.Save(); err != nil {
			a.addLog("Error al guardar configuración: " + err.Error())
			ShowError(a.window, "Error", "No se pudo guardar la configuración")
			return
		}

		// Actualizar UI
		a.updateConfigStatus()
		fileName := filepath.Base(filePath)
		a.addLog(fmt.Sprintf("✓ Archivo VPN seleccionado: %s", fileName))
		ShowInfo(a.window, "Archivo configurado", fmt.Sprintf("Se ha configurado el archivo:\n%s\n\nYa puedes conectarte.", fileName))
	}, a.window)

	// Configurar filtro para solo mostrar archivos .ovpn
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".ovpn"}))

	// Intentar abrir en el directorio home del usuario
	homeDir, err := storage.ListerForURI(storage.NewFileURI(getUserHomeDir()))
	if err == nil {
		fileDialog.SetLocation(homeDir)
	}

	fileDialog.Show()
}

// getUserHomeDir retorna el directorio home del usuario
func getUserHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}

// Run inicia la aplicación con soporte de system tray
func (a *App) Run() {
	// Iniciar systray en un goroutine
	// Systray.Run es bloqueante, por lo que lo ejecutamos en paralelo
	go func() {
		a.trayIcon.Run(
			func() {
				// onReady - tray está listo e inicializado
				a.addLog("System tray inicializado")
				// Ahora sí actualizar el icono del tray
				a.updateTrayIcon()
			},
			func() {
				// onExit - tray cerrado
				a.fyneApp.Quit()
			},
		)
	}()

	// Iniciar la ventana de Fyne (bloqueante)
	a.window.ShowAndRun()

	// Cuando la ventana de Fyne se cierra, cerrar también el tray
	if a.trayIcon != nil {
		a.trayIcon.Quit()
	}
}
