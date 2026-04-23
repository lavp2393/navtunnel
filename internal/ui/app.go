package ui

import (
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/lavp2393/navtunnel/internal/config"
	"github.com/lavp2393/navtunnel/internal/core"
	"github.com/lavp2393/navtunnel/internal/logs"
	"github.com/lavp2393/navtunnel/internal/tray"
)

// App es la aplicación principal: arma la ventana Fyne con el dashboard,
// integra el tray nativo y controla el Manager de core.
type App struct {
	fyneApp fyne.App
	window  fyne.Window

	cfg     *config.Config
	logBuf  *logs.Buffer
	tray    *tray.Systray

	mu      sync.Mutex
	manager *core.Manager
	sendFns core.SendFns

	credMu        sync.Mutex
	savedUser     string
	savedPass     string
	rememberCreds bool
	credStore     core.CredentialStoreMethod

	// Widgets del dashboard.
	statusDot    *canvas.Circle
	statusLabel  *widget.Label
	connectBtn   *widget.Button
	disconBtn    *widget.Button
	ovpnEntry    *widget.Entry
	rxTotal      *widget.Label
	txTotal      *widget.Label
	rxRate       *widget.Label
	txRate       *widget.Label
	tunLabel     *widget.Label
	serverLabel  *widget.Label
	chart        *TrafficChart
	logsBox      *widget.Entry

	// Modal de credenciales activo (para evitar múltiples abiertos al mismo
	// tiempo y cancelar si el flow cambia).
	promptMu     sync.Mutex
	promptActive bool
}

// NewApp construye y configura la aplicación sin arrancarla todavía.
func NewApp() *App {
	a := &App{
		fyneApp: fyneApp.NewWithID("com.preyhq.navtunnel"),
		logBuf:  logs.NewBuffer(400),
	}
	a.fyneApp.Settings().SetTheme(&cyberpunkTheme{})

	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
		// No es crítico: seguimos con default y el usuario corrige desde la UI.
		a.logBuf.Add("Aviso: " + err.Error())
	}
	if cfg == nil {
		cfg = config.Default()
	}
	a.cfg = cfg

	a.window = a.fyneApp.NewWindow("NavTunnel")
	a.window.Resize(fyne.NewSize(920, 720))
	a.window.SetCloseIntercept(func() { a.window.Hide() })

	a.buildUI()
	a.loadStoredCredentials()
	a.setupTray()

	return a
}

// Run arranca el tray en goroutine y Fyne en el main thread (requerido por
// macOS); bloquea hasta que el usuario elige Salir.
func (a *App) Run() {
	go a.tray.Run(func() {
		a.setStatus("Desconectado", statusIdle)
	}, nil)

	a.window.Show()
	a.fyneApp.Run()

	if a.tray != nil {
		a.tray.Quit()
	}
	a.disconnect()
}

// --- UI ---------------------------------------------------------------------

func (a *App) buildUI() {
	// Header: logo + estado.
	a.statusDot = canvas.NewCircle(colorDim)
	a.statusDot.Resize(fyne.NewSize(12, 12))
	a.statusDot.StrokeWidth = 0
	statusDotBox := container.NewWithoutLayout(a.statusDot)
	statusDotBox.Resize(fyne.NewSize(14, 14))

	a.statusLabel = widget.NewLabel("DESCONECTADO")
	a.statusLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	title := canvas.NewText("NAVTUNNEL", colorCyan)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 16
	subtitle := canvas.NewText("SECURE CHANNEL", colorDim)
	subtitle.TextSize = 9
	subtitle.TextStyle = fyne.TextStyle{Monospace: true}

	header := container.NewBorder(
		nil, nil,
		container.NewHBox(
			canvas.NewText("◈", colorCyan),
			container.NewVBox(title, subtitle),
		),
		container.NewHBox(statusDotBox, a.statusLabel),
		nil,
	)

	// Chart de tráfico.
	a.chart = NewTrafficChart(fyne.NewSize(600, 220))
	chartTitle := canvas.NewText("TRÁFICO EN VIVO", colorCyan)
	chartTitle.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	chartTitle.TextSize = 11

	a.rxRate = newMetricValue("0 B/s", colorCyan)
	a.txRate = newMetricValue("0 B/s", colorMagenta)
	rateRow := container.NewGridWithColumns(2,
		newMetricBox("↓ RX RATE", a.rxRate),
		newMetricBox("↑ TX RATE", a.txRate),
	)

	chartCard := newCard(container.NewBorder(
		chartTitle, rateRow, nil, nil,
		a.chart,
	))

	// Grid 2x2 de métricas totales.
	a.rxTotal = newMetricValue("0 B", nil)
	a.txTotal = newMetricValue("0 B", nil)
	a.tunLabel = newMetricValue("—", nil)
	a.serverLabel = newMetricValue("—", nil)

	metricGrid := container.NewGridWithColumns(4,
		newCard(newMetricBox("TOTAL RX", a.rxTotal)),
		newCard(newMetricBox("TOTAL TX", a.txTotal)),
		newCard(newMetricBox("TUN IP", a.tunLabel)),
		newCard(newMetricBox("SERVIDOR", a.serverLabel)),
	)

	// Controls: conectar/desconectar + config.
	a.connectBtn = widget.NewButton("◉ CONECTAR", a.onConnect)
	a.connectBtn.Importance = widget.HighImportance
	a.disconBtn = widget.NewButton("⊗ DESCONECTAR", a.onDisconnect)
	a.disconBtn.Disable()

	a.ovpnEntry = widget.NewEntry()
	a.ovpnEntry.PlaceHolder = "/ruta/al/archivo.ovpn"
	a.ovpnEntry.SetText(a.cfg.VPNConfigPath)

	browseBtn := widget.NewButton("EXAMINAR", a.onBrowse)
	saveBtn := widget.NewButton("GUARDAR", a.onSaveConfig)

	btnRow := container.NewHBox(a.connectBtn, a.disconBtn)
	cfgRow := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("OVPN", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true, Bold: true}),
		container.NewHBox(browseBtn, saveBtn),
		a.ovpnEntry,
	)
	controlsCard := newCard(container.NewVBox(btnRow, widget.NewSeparator(), cfgRow))

	// Dashboard container.
	dashboard := container.NewVBox(
		chartCard,
		metricGrid,
		controlsCard,
	)

	// Logs tab.
	a.logsBox = widget.NewMultiLineEntry()
	a.logsBox.Disable()
	a.logsBox.TextStyle = fyne.TextStyle{Monospace: true}
	a.logsBox.Wrapping = fyne.TextWrapOff
	logsScroll := container.NewScroll(a.logsBox)
	logsCard := newCard(container.NewBorder(
		container.NewHBox(
			monoTitle("LOG STREAM"),
			layout.NewSpacer(),
			widget.NewButton("LIMPIAR", func() {
				a.logBuf.Clear()
				a.logsBox.SetText("")
			}),
		),
		nil, nil, nil, logsScroll,
	))

	tabs := container.NewAppTabs(
		container.NewTabItem("Dashboard", container.NewPadded(dashboard)),
		container.NewTabItem("Logs", container.NewPadded(logsCard)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	root := container.NewBorder(
		container.NewPadded(header),
		nil, nil, nil,
		tabs,
	)
	a.window.SetContent(root)
}

// newCard envuelve el contenido con un borde sutil para "cards" tipo panel.
func newCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colorPanel)
	bg.StrokeColor = color.NRGBA{0x78, 0xDC, 0xFF, 0x2E}
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// newMetricBox arma un bloque (etiqueta monoespaciada + valor).
func newMetricBox(label string, value *widget.Label) fyne.CanvasObject {
	l := canvas.NewText(label, colorDim)
	l.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	l.TextSize = 10
	return container.NewVBox(l, value)
}

// newMetricValue construye un label estilo display numérico.
func newMetricValue(text string, c color.Color) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return lbl
}

func monoTitle(s string) fyne.CanvasObject {
	t := canvas.NewText(s, colorCyan)
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.TextSize = 11
	return t
}

// --- Tray -------------------------------------------------------------------

func (a *App) setupTray() {
	a.tray = tray.NewSystray(tray.MenuCallbacks{
		OnConnect:    func() { fyne.Do(a.onConnect) },
		OnDisconnect: func() { fyne.Do(a.onDisconnect) },
		OnShowWindow: func() { fyne.Do(func() { a.window.Show() }) },
		OnQuit: func() {
			fyne.Do(func() {
				a.disconnect()
				a.fyneApp.Quit()
			})
		},
	})
}

// --- Status / métricas ------------------------------------------------------

type statusKind int

const (
	statusIdle statusKind = iota
	statusWait
	statusOK
	statusErr
)

func (a *App) setStatus(text string, kind statusKind) {
	fyne.Do(func() {
		a.statusLabel.SetText(strings.ToUpper(text))
		switch kind {
		case statusOK:
			a.statusDot.FillColor = color.NRGBA{0x00, 0xFF, 0xA3, 0xFF}
			a.tray.SetIcon(tray.IconConnected)
			a.tray.UpdateState(text, true)
		case statusWait:
			a.statusDot.FillColor = color.NRGBA{0xFF, 0xB8, 0x4D, 0xFF}
			a.tray.SetIcon(tray.IconConnecting)
			a.tray.UpdateState(text, false)
		case statusErr:
			a.statusDot.FillColor = color.NRGBA{0xFF, 0x4D, 0x6D, 0xFF}
			a.tray.SetIcon(tray.IconError)
			a.tray.UpdateState(text, false)
		default:
			a.statusDot.FillColor = colorDim
			a.tray.SetIcon(tray.IconDisconnected)
			a.tray.UpdateState(text, false)
		}
		canvas.Refresh(a.statusDot)
	})
}

// --- Acciones ---------------------------------------------------------------

func (a *App) onConnect() {
	if !a.cfg.IsVPNConfigValid() {
		dialog.ShowError(errors.New("seleccioná un archivo .ovpn válido primero"), a.window)
		return
	}

	a.mu.Lock()
	if a.manager != nil {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	openvpnPath, err := core.FindOpenVPN()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.addLog("Usando openvpn: " + openvpnPath)
	a.setStatus("Conectando...", statusWait)

	a.connectBtn.Disable()
	a.disconBtn.Enable()

	mgr, err := core.Start(a.cfg.VPNConfigPath, openvpnPath)
	if err != nil {
		a.addLog("Error al iniciar: " + err.Error())
		dialog.ShowError(err, a.window)
		a.setStatus("Error", statusErr)
		a.connectBtn.Enable()
		a.disconBtn.Disable()
		return
	}

	a.mu.Lock()
	a.manager = mgr
	a.sendFns = mgr.SendFunctions()
	a.mu.Unlock()

	go a.pumpEvents(mgr)
}

func (a *App) onDisconnect() {
	a.setStatus("Desconectando...", statusWait)
	a.disconBtn.Disable()
	a.connectBtn.Disable()
	go a.disconnect()
}

func (a *App) disconnect() {
	a.mu.Lock()
	mgr := a.manager
	a.manager = nil
	a.sendFns = core.SendFns{}
	a.mu.Unlock()
	if mgr != nil {
		mgr.Stop()
	}
	fyne.Do(func() {
		a.setStatus("Desconectado", statusIdle)
		a.connectBtn.Enable()
		a.disconBtn.Disable()
		a.chart.Reset()
		a.rxRate.SetText("0 B/s")
		a.txRate.SetText("0 B/s")
	})
}

func (a *App) onBrowse() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		path := reader.URI().Path()
		if filepath.Ext(strings.ToLower(path)) != ".ovpn" {
			dialog.ShowError(errors.New("el archivo debe ser .ovpn"), a.window)
			return
		}
		a.ovpnEntry.SetText(path)
		a.saveConfig(path)
	}, a.window)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".ovpn"}))
	d.Resize(fyne.NewSize(700, 500))
	d.Show()
}

func (a *App) onSaveConfig() {
	a.saveConfig(strings.TrimSpace(a.ovpnEntry.Text))
}

func (a *App) saveConfig(path string) {
	if path == "" {
		return
	}
	a.cfg.VPNConfigPath = path
	if err := a.cfg.Save(); err != nil {
		a.addLog("Error guardando config: " + err.Error())
		return
	}
	a.addLog("✓ Configuración guardada: " + path)
}

// --- Pump de eventos del Manager --------------------------------------------

func (a *App) pumpEvents(mgr *core.Manager) {
	for ev := range mgr.Events() {
		a.routeEvent(ev)
	}
	// Canal cerrado: el Manager terminó.
	fyne.Do(func() {
		a.setStatus("Desconectado", statusIdle)
		a.connectBtn.Enable()
		a.disconBtn.Disable()
	})
}

func (a *App) routeEvent(ev core.Event) {
	switch ev.Type {
	case core.EventLogLine:
		a.addLog(ev.Message)

	case core.EventState:
		switch ev.State {
		case "CONNECTED":
			a.setStatus("Conectado", statusOK)
		case "EXITING":
			a.setStatus("Saliendo", statusWait)
		default:
			a.setStatus(ev.State, statusWait)
		}
		if ev.LocalTunIP != "" {
			fyne.Do(func() { a.tunLabel.SetText(ev.LocalTunIP) })
		}
		if ev.RemoteIP != "" {
			fyne.Do(func() { a.serverLabel.SetText(ev.RemoteIP) })
		}

	case core.EventBytecount:
		rate := a.currentRates()
		fyne.Do(func() {
			a.rxTotal.SetText(humanBytes(ev.BytesIn))
			a.txTotal.SetText(humanBytes(ev.BytesOut))
			a.rxRate.SetText(humanBytes(rate.in) + "/s")
			a.txRate.SetText(humanBytes(rate.out) + "/s")
		})
		a.chart.Push(float64(rate.in), float64(rate.out))

	case core.EventAskUser:
		a.credMu.Lock()
		saved := a.savedUser
		a.credMu.Unlock()
		if saved != "" && a.autoSendUsername(saved) {
			return
		}
		a.openCredentialPrompt("USUARIO", ev.Message, false, true, saved != "", func(r credentialResult) {
			a.credMu.Lock()
			a.savedUser = r.value
			a.rememberCreds = r.remember
			a.credMu.Unlock()
			a.mu.Lock()
			fn := a.sendFns.Username
			a.mu.Unlock()
			if fn != nil {
				if err := fn(r.value); err != nil {
					a.addLog("Error enviando usuario: " + err.Error())
				}
			}
		})

	case core.EventAskPass:
		a.credMu.Lock()
		saved := a.savedPass
		a.credMu.Unlock()
		if saved != "" && a.autoSendPassword(saved) {
			return
		}
		a.openCredentialPrompt("CONTRASEÑA", ev.Message, true, true, a.rememberCreds, func(r credentialResult) {
			a.credMu.Lock()
			a.savedPass = r.value
			a.rememberCreds = r.remember
			user := a.savedUser
			a.credMu.Unlock()
			a.persistCredentials(user, r.value, r.remember)
			a.mu.Lock()
			fn := a.sendFns.Password
			a.mu.Unlock()
			if fn != nil {
				if err := fn(r.value); err != nil {
					a.addLog("Error enviando contraseña: " + err.Error())
				}
			}
		})

	case core.EventAskOTP:
		a.openCredentialPrompt("CÓDIGO OTP", ev.Message, false, false, false, func(r credentialResult) {
			a.mu.Lock()
			fn := a.sendFns.OTP
			a.mu.Unlock()
			if fn != nil {
				if err := fn(r.value); err != nil {
					a.addLog("Error enviando OTP: " + err.Error())
				}
			}
		})

	case core.EventConnected:
		a.setStatus("Conectado", statusOK)
		a.addLog(ev.Message)

	case core.EventAuthFailed:
		a.invalidateSavedFor(ev.Stage)
		a.addLog("✗ " + ev.Message)

	case core.EventFatal:
		a.addLog("FATAL: " + ev.Message)
		a.setStatus("Error", statusErr)
		dialog.ShowError(fmt.Errorf("%s", ev.Message), a.window)

	case core.EventDisconnected:
		a.addLog(ev.Message)
		fyne.Do(func() {
			a.setStatus("Desconectado", statusIdle)
			a.connectBtn.Enable()
			a.disconBtn.Disable()
		})
	}
}

// openCredentialPrompt garantiza que solo haya un modal abierto.
func (a *App) openCredentialPrompt(title, msg string, masked, showRemember, initialRemember bool, onConfirm func(credentialResult)) {
	a.promptMu.Lock()
	if a.promptActive {
		a.promptMu.Unlock()
		return
	}
	a.promptActive = true
	a.promptMu.Unlock()

	fyne.Do(func() {
		promptCredential(a.window, title, msg, masked, showRemember, initialRemember, func(r credentialResult) {
			a.promptMu.Lock()
			a.promptActive = false
			a.promptMu.Unlock()
			onConfirm(r)
		})
	})
}

// --- Credenciales guardadas -------------------------------------------------

type rates struct{ in, out uint64 }

func (a *App) currentRates() rates {
	a.mu.Lock()
	mgr := a.manager
	a.mu.Unlock()
	if mgr == nil {
		return rates{}
	}
	m := mgr.Metrics()
	return rates{in: m.BytesInRate, out: m.BytesOutRate}
}

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
	a.savedUser = user
	a.savedPass = pass
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
	if err := core.DeleteCredentials(); err != nil {
		a.addLog("No se pudieron borrar credenciales: " + err.Error())
	}
	a.credMu.Lock()
	a.savedUser = ""
	a.savedPass = ""
	a.credStore = core.CredentialStoreMethodNone
	a.credMu.Unlock()
}

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

func (a *App) invalidateSavedFor(stage string) {
	a.credMu.Lock()
	switch stage {
	case "username":
		a.savedUser = ""
		a.savedPass = ""
	case "password":
		a.savedPass = ""
	default:
		a.credMu.Unlock()
		return
	}
	a.credMu.Unlock()
	go func() { _ = core.DeleteCredentials() }()
}

// --- Logs -------------------------------------------------------------------

func (a *App) addLog(line string) {
	a.logBuf.Add(line)
	if a.logsBox == nil {
		return
	}
	text := a.logBuf.GetText()
	fyne.Do(func() {
		a.logsBox.SetText(text)
		a.logsBox.CursorRow = len(a.logBuf.GetAll())
	})
}

// --- Helpers ----------------------------------------------------------------

func humanBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}
