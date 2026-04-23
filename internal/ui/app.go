package ui

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/lavp2393/navtunnel/internal/config"
	"github.com/lavp2393/navtunnel/internal/core"
	"github.com/lavp2393/navtunnel/internal/logs"
	"github.com/lavp2393/navtunnel/internal/tray"
)

// App orquesta el ciclo de vida de NavTunnel: carga configuración, inicia
// el HTTP local con la UI embebida, monta el tray nativo y conecta el
// Manager de core para manejar la conexión OpenVPN.
type App struct {
	cfg     *config.Config
	logBuf  *logs.Buffer
	hub     *eventHub
	tray    *tray.Systray
	server  *server

	mu      sync.Mutex
	manager *core.Manager
	sendFns core.SendFns

	credMu        sync.Mutex
	savedUsername string
	savedPassword string
	rememberCreds bool
	credStore     core.CredentialStoreMethod
}

// NewApp construye la aplicación. No arranca nada hasta Run().
func NewApp() *App {
	a := &App{
		logBuf: logs.NewBuffer(200),
		hub:    newEventHub(),
	}

	cfg, err := config.Load()
	if err != nil {
		if !errors.Is(err, config.ErrConfigNotFound) {
			log.Printf("config: %v", err)
		}
		cfg = config.Default()
	}
	a.cfg = cfg

	a.loadStoredCredentials()
	return a
}

// Run monta el servidor HTTP local, abre el tray y bloquea hasta que el
// usuario elige "Salir" desde el menú. El systray.Run() retiene el main
// thread (requerido por macOS) — por eso los demás componentes corren en
// goroutines.
func (a *App) Run() {
	srv, err := newServer(a)
	if err != nil {
		log.Fatalf("no se pudo iniciar la UI web: %v", err)
	}
	a.server = srv
	a.addLog(fmt.Sprintf("UI lista en %s", srv.url()))

	a.tray = tray.NewSystray(tray.MenuCallbacks{
		OnConnect:    a.onTrayConnect,
		OnDisconnect: a.disconnect,
		OnShowWindow: a.openPanel,
		OnQuit:       a.onQuit,
	})

	a.tray.Run(
		func() {
			a.addLog("Listo. Usá el menú del tray o el panel web.")
			a.openPanel()
		},
		func() {
			// onExit — el runtime de systray ya está cerrando.
		},
	)

	a.shutdown()
}

// --- Acciones expuestas al server (y al tray) ------------------------------

func (a *App) currentOvpnPath() string {
	if a.cfg == nil {
		return ""
	}
	return a.cfg.VPNConfigPath
}

func (a *App) currentMetrics() metrics {
	a.mu.Lock()
	mgr := a.manager
	a.mu.Unlock()
	if mgr == nil {
		return metrics{}
	}
	m := mgr.Metrics()
	return metrics{
		State:          m.State,
		BytesIn:        m.BytesIn,
		BytesOut:       m.BytesOut,
		BytesInRate:    m.BytesInRate,
		BytesOutRate:   m.BytesOutRate,
		LocalTunnelIP:  m.LocalTunnelIP,
		RemoteServerIP: m.RemoteServerIP,
	}
}

func (a *App) recentLogs() []string { return a.logBuf.GetAll() }

// connect arranca el Manager si hay un .ovpn configurado y no hay sesión
// activa. Devuelve error al llamador (el server lo traduce a 400).
func (a *App) connect() error {
	if a.cfg == nil || !a.cfg.IsVPNConfigValid() {
		return errors.New("archivo .ovpn no configurado o inaccesible")
	}

	a.mu.Lock()
	if a.manager != nil {
		a.mu.Unlock()
		return errors.New("ya hay una conexión activa")
	}
	a.mu.Unlock()

	openvpnPath, err := core.FindOpenVPN()
	if err != nil {
		return err
	}
	a.addLog("Usando openvpn: " + openvpnPath)

	mgr, err := core.Start(a.cfg.VPNConfigPath, openvpnPath)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.manager = mgr
	a.sendFns = mgr.SendFunctions()
	a.mu.Unlock()

	a.updateTray(tray.IconConnecting, "Conectando...", false)
	go a.pumpEvents(mgr)
	return nil
}

// disconnect para la sesión actual si existe. No retorna error.
func (a *App) disconnect() {
	a.mu.Lock()
	mgr := a.manager
	a.manager = nil
	a.sendFns = core.SendFns{}
	a.mu.Unlock()

	if mgr != nil {
		a.addLog("Desconectando...")
		mgr.Stop()
	}
	a.updateTray(tray.IconDisconnected, "Desconectado", false)
}

// submitCredential entrega un valor de credencial a la etapa correspondiente
// del Manager. Se invoca desde el formulario modal del cliente web.
func (a *App) submitCredential(stage, value string, remember bool) error {
	a.mu.Lock()
	fns := a.sendFns
	a.mu.Unlock()

	a.credMu.Lock()
	a.rememberCreds = remember
	a.credMu.Unlock()

	switch stage {
	case "user":
		a.credMu.Lock()
		a.savedUsername = value
		a.credMu.Unlock()
		if fns.Username == nil {
			return errors.New("manager no preparado para credenciales")
		}
		return fns.Username(value)
	case "pass":
		a.credMu.Lock()
		a.savedPassword = value
		shouldRemember := a.rememberCreds
		user := a.savedUsername
		a.credMu.Unlock()
		a.persistCredentials(user, value, shouldRemember)
		if fns.Password == nil {
			return errors.New("manager no preparado para credenciales")
		}
		return fns.Password(value)
	case "otp":
		if fns.OTP == nil {
			return errors.New("manager no preparado para credenciales")
		}
		return fns.OTP(value)
	}
	return fmt.Errorf("etapa desconocida: %q", stage)
}

// saveOvpnPath persiste la ruta seleccionada por el usuario en la config.
func (a *App) saveOvpnPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("ruta vacía")
	}
	if !strings.HasSuffix(strings.ToLower(path), ".ovpn") {
		return errors.New("el archivo debe tener extensión .ovpn")
	}
	if a.cfg == nil {
		a.cfg = config.Default()
	}
	a.cfg.VPNConfigPath = path
	return a.cfg.Save()
}

// --- Tray callbacks --------------------------------------------------------

func (a *App) onTrayConnect() {
	if err := a.connect(); err != nil {
		a.addLog("Error al conectar: " + err.Error())
	}
}

func (a *App) openPanel() {
	if a.server == nil {
		return
	}
	url := a.server.url()
	if err := spawnPanel(url); err != nil {
		a.addLog("No se pudo abrir la ventana nativa: " + err.Error())
		if fbErr := openBrowserFallback(url); fbErr != nil {
			a.addLog("Tampoco se pudo abrir el navegador: " + fbErr.Error())
			a.addLog("Abrí manualmente: " + url)
		}
	}
}

func (a *App) onQuit() {
	a.disconnect()
	a.shutdown()
}

// --- Pump de eventos del Manager -------------------------------------------

func (a *App) pumpEvents(mgr *core.Manager) {
	for ev := range mgr.Events() {
		a.routeEvent(ev)
	}
	// Canal cerrado: el Manager terminó. Asegurar UI coherente.
	a.mu.Lock()
	if a.manager == mgr {
		a.manager = nil
		a.sendFns = core.SendFns{}
	}
	a.mu.Unlock()
	a.updateTray(tray.IconDisconnected, "Desconectado", false)
}

func (a *App) routeEvent(ev core.Event) {
	switch ev.Type {
	case core.EventLogLine:
		a.addLog(ev.Message)

	case core.EventState:
		a.hub.broadcast(hubEvent{
			Type:       "state",
			State:      ev.State,
			LocalTunIP: ev.LocalTunIP,
			RemoteIP:   ev.RemoteIP,
		})
		switch ev.State {
		case "CONNECTED":
			a.updateTray(tray.IconConnected, "Conectado", true)
		case "EXITING":
			a.updateTray(tray.IconDisconnected, "Desconectado", false)
		default:
			a.updateTray(tray.IconConnecting, ev.State, false)
		}

	case core.EventBytecount:
		m := a.currentMetrics()
		a.hub.broadcast(hubEvent{
			Type:     "bytecount",
			BytesIn:  ev.BytesIn,
			BytesOut: ev.BytesOut,
			RateIn:   m.BytesInRate,
			RateOut:  m.BytesOutRate,
		})

	case core.EventAskUser:
		a.credMu.Lock()
		saved := a.savedUsername
		a.credMu.Unlock()
		if saved != "" && a.autoSendUsername(saved) {
			return
		}
		a.hub.broadcast(hubEvent{Type: "ask-user", Message: ev.Message + prefillHint(saved)})

	case core.EventAskPass:
		a.credMu.Lock()
		saved := a.savedPassword
		a.credMu.Unlock()
		if saved != "" && a.autoSendPassword(saved) {
			return
		}
		a.hub.broadcast(hubEvent{Type: "ask-pass", Message: ev.Message})

	case core.EventAskOTP:
		a.hub.broadcast(hubEvent{Type: "ask-otp", Message: ev.Message})

	case core.EventConnected:
		a.hub.broadcast(hubEvent{Type: "connected"})
		a.addLog(ev.Message)

	case core.EventAuthFailed:
		// Si el fallo vino con credenciales persistidas, invalidarlas para
		// que el próximo ask-user/pass pregunte al usuario en vez de reenviar
		// las mismas credenciales malas en loop.
		a.invalidateSavedFor(ev.Stage)
		a.hub.broadcast(hubEvent{Type: "auth-failed", Message: ev.Message, Stage: ev.Stage})
		a.addLog("✗ " + ev.Message)

	case core.EventFatal:
		a.hub.broadcast(hubEvent{Type: "fatal", Message: ev.Message})
		a.addLog("FATAL: " + ev.Message)

	case core.EventDisconnected:
		a.hub.broadcast(hubEvent{Type: "disconnected", Message: ev.Message})
		a.addLog(ev.Message)
		a.updateTray(tray.IconDisconnected, "Desconectado", false)
	}
}

// prefillHint agrega una sugerencia visual en el mensaje del modal sin
// preencher el input (por seguridad: el JS no recibe la contraseña guardada,
// solo el username en el log).
func prefillHint(user string) string {
	if user == "" {
		return ""
	}
	return " (último: " + user + ")"
}

// autoSendUsername envía directamente el username guardado sin mostrar el
// modal al usuario. Devuelve true si el envío salió bien; false si hay que
// caer al prompt manual (p.ej. Manager no listo, error de socket).
func (a *App) autoSendUsername(v string) bool {
	a.mu.Lock()
	fn := a.sendFns.Username
	a.mu.Unlock()
	if fn == nil {
		return false
	}
	if err := fn(v); err != nil {
		a.addLog("No se pudo enviar usuario guardado: " + err.Error())
		return false
	}
	a.addLog("✓ Usuario enviado (recordado)")
	return true
}

// autoSendPassword envía directamente la contraseña guardada. Mismo
// contrato que autoSendUsername.
func (a *App) autoSendPassword(v string) bool {
	a.mu.Lock()
	fn := a.sendFns.Password
	a.mu.Unlock()
	if fn == nil {
		return false
	}
	if err := fn(v); err != nil {
		a.addLog("No se pudo enviar contraseña guardada: " + err.Error())
		return false
	}
	a.addLog("✓ Contraseña enviada (recordada)")
	return true
}

// invalidateSavedFor borra de memoria y del storage persistente las
// credenciales que causaron un AUTH_FAILED. No toca la bandera "recordar":
// si el usuario ingresa credenciales nuevas correctas, volverán a guardarse.
func (a *App) invalidateSavedFor(stage string) {
	a.credMu.Lock()
	switch stage {
	case "username":
		a.savedUsername = ""
		a.savedPassword = ""
	case "password":
		a.savedPassword = ""
	default:
		a.credMu.Unlock()
		return
	}
	a.credMu.Unlock()
	go func() { _ = core.DeleteCredentials() }()
}

// --- Credenciales ---------------------------------------------------------

func (a *App) loadStoredCredentials() {
	user, pass, method, warning, err := core.LoadCredentials()
	if warning != "" {
		a.addLog("Aviso credenciales: " + warning)
	}
	if err != nil {
		if !errors.Is(err, core.ErrCredentialsNotFound) {
			a.addLog("No se pudieron cargar credenciales: " + err.Error())
		}
		return
	}
	a.credMu.Lock()
	a.savedUsername = user
	a.savedPassword = pass
	a.credStore = method
	a.rememberCreds = true
	a.credMu.Unlock()
}

func (a *App) persistCredentials(user, pass string, remember bool) {
	if remember {
		if user == "" || pass == "" {
			return
		}
		method, warning, err := core.SaveCredentials(user, pass)
		if err != nil {
			a.addLog("No se pudieron guardar credenciales: " + err.Error())
			return
		}
		if warning != "" {
			a.addLog("Aviso credenciales: " + warning)
		}
		a.credMu.Lock()
		a.credStore = method
		a.credMu.Unlock()
		return
	}
	// No recordar: borrar cualquier credencial previa.
	if err := core.DeleteCredentials(); err != nil {
		a.addLog("No se pudieron borrar credenciales guardadas: " + err.Error())
	}
	a.credMu.Lock()
	a.savedUsername = ""
	a.savedPassword = ""
	a.credStore = core.CredentialStoreMethodNone
	a.credMu.Unlock()
}

// --- Helpers --------------------------------------------------------------

func (a *App) addLog(line string) {
	a.logBuf.Add(line)
	a.hub.broadcast(hubEvent{Type: "log", Message: line})
}

func (a *App) updateTray(icon tray.IconType, text string, connected bool) {
	if a.tray == nil {
		return
	}
	a.tray.SetIcon(icon)
	a.tray.UpdateState(text, connected)
}

func (a *App) shutdown() {
	a.disconnect()
	if a.server != nil {
		a.server.shutdown()
	}
}
