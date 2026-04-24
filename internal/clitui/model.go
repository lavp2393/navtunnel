package clitui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lavp2393/navtunnel/internal/daemon"
)

// Run monta la TUI bubbletea y bloquea hasta que el usuario salga (q).
// No cierra el daemon — la conexión OpenVPN sigue en background.
func Run(c *daemon.Client) error {
	m := newModel(c)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- Mensajes internos ------------------------------------------------------

type daemonEventMsg struct{ Envelope daemon.Envelope }
type daemonClosedMsg struct{}
type tickMsg time.Time

// --- Estado del modelo ------------------------------------------------------

type tab int

const (
	tabDashboard tab = iota
	tabLogs
)

type promptState struct {
	stage   string // "user" | "pass" | "otp"
	message string
	masked  bool
}

type model struct {
	client *daemon.Client

	width, height int
	active        tab

	state    string
	tunIP    string
	remoteIP string
	bytesIn  uint64
	bytesOut uint64
	rateIn   uint64
	rateOut  uint64
	ovpn     string
	version  string

	rxBuf []float64
	txBuf []float64

	logs []string

	prompt *promptState
	input  textinput.Model

	rememberCreds bool
	err           string
}

func newModel(c *daemon.Client) model {
	in := textinput.New()
	in.Prompt = "▸ "
	in.Placeholder = "..."
	in.CharLimit = 256
	in.PromptStyle = cyanStyle
	in.TextStyle = valueStyle

	return model{
		client:        c,
		rxBuf:         make([]float64, 0, 120),
		txBuf:         make([]float64, 0, 120),
		input:         in,
		state:         "DESCONECTADO",
		rememberCreds: true,
	}
}

// --- Init & Update ----------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.waitEvent(),
		tickEvery(),
	)
}

// waitEvent espera el siguiente mensaje del daemon y lo envuelve en un msg
// bubbletea. Se re-arma en cada Update.
func (m model) waitEvent() tea.Cmd {
	return func() tea.Msg {
		env, ok := <-m.client.Events
		if !ok {
			return daemonClosedMsg{}
		}
		return daemonEventMsg{Envelope: env}
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case daemonEventMsg:
		m = m.handleEvent(msg.Envelope)
		return m, m.waitEvent()

	case daemonClosedMsg:
		m.err = "Conexión con el daemon perdida. Cerrá con 'q' y volvé a ejecutar navtunnel-cli."
		return m, nil

	case tickMsg:
		// redraw periódico para que los rates "envejezcan" si no llegan updates.
		return m, tickEvery()
	}
	return m, nil
}

// handleKey procesa teclas; distingue entre modo normal y modo prompt.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt != nil {
		switch msg.Type {
		case tea.KeyEsc:
			m.prompt = nil
			m.input.Reset()
			return m, nil
		case tea.KeyEnter:
			value := m.input.Value()
			m.input.Reset()
			stage := m.prompt.stage
			m.prompt = nil
			return m, m.submitCredential(stage, value)
		case tea.KeyCtrlR:
			m.rememberCreds = !m.rememberCreds
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "c":
		return m, m.sendCmd(daemon.CmdConnect, nil)
	case "d":
		return m, m.sendCmd(daemon.CmdDisconnect, nil)
	case "s":
		return m, m.sendCmd(daemon.CmdShutdown, nil)
	case "tab":
		if m.active == tabDashboard {
			m.active = tabLogs
		} else {
			m.active = tabDashboard
		}
		return m, nil
	case "1":
		m.active = tabDashboard
		return m, nil
	case "2":
		m.active = tabLogs
		return m, nil
	}
	return m, nil
}

// handleEvent procesa un envelope del daemon y devuelve el modelo modificado.
func (m model) handleEvent(env daemon.Envelope) model {
	switch env.Type {
	case daemon.EvtHello:
		var p daemon.HelloPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.version = p.Version
		m.ovpn = p.OvpnPath
	case daemon.EvtState:
		var p daemon.StatePayload
		_ = json.Unmarshal(env.Payload, &p)
		if p.State != "" {
			m.state = p.State
		}
		if p.LocalTunIP != "" {
			m.tunIP = p.LocalTunIP
		}
		if p.RemoteServerIP != "" {
			m.remoteIP = p.RemoteServerIP
		}
		m.bytesIn = p.BytesIn
		m.bytesOut = p.BytesOut
		m.rateIn = p.BytesInRate
		m.rateOut = p.BytesOutRate
	case daemon.EvtBytecount:
		var p daemon.BytecountPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.bytesIn = p.BytesIn
		m.bytesOut = p.BytesOut
		m.rateIn = p.RateIn
		m.rateOut = p.RateOut
		m.rxBuf = appendSample(m.rxBuf, float64(p.RateIn), 120)
		m.txBuf = appendSample(m.txBuf, float64(p.RateOut), 120)
	case daemon.EvtLog:
		var p daemon.LogPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.logs = appendLog(m.logs, p.Line, 400)
	case daemon.EvtAskUser:
		var p daemon.AskPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.startPrompt("user", p.Message, false)
	case daemon.EvtAskPass:
		var p daemon.AskPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.startPrompt("pass", p.Message, true)
	case daemon.EvtAskOTP:
		var p daemon.AskPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.startPrompt("otp", p.Message, false)
	case daemon.EvtConnected:
		m.state = "CONNECTED"
	case daemon.EvtDisconnect:
		var p daemon.DisconnectedPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.state = "DESCONECTADO"
		m.rateIn = 0
		m.rateOut = 0
	case daemon.EvtAuthFailed:
		var p daemon.AuthFailedPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.logs = appendLog(m.logs, "✗ "+p.Message, 400)
	case daemon.EvtFatal:
		var p daemon.FatalPayload
		_ = json.Unmarshal(env.Payload, &p)
		m.err = p.Message
	case daemon.EvtReply:
		var p daemon.ReplyPayload
		_ = json.Unmarshal(env.Payload, &p)
		if !p.OK && p.Error != "" {
			m.logs = appendLog(m.logs, "✗ "+p.Error, 400)
		}
	}
	return m
}

// startPrompt prepara la textinput y activa el overlay de prompt.
func (m *model) startPrompt(stage, message string, masked bool) {
	m.prompt = &promptState{stage: stage, message: message, masked: masked}
	m.input.Reset()
	if masked {
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
	} else {
		m.input.EchoMode = textinput.EchoNormal
	}
	m.input.Focus()
}

// sendCmd encapsula client.Send como tea.Cmd.
func (m model) sendCmd(cmd string, payload any) tea.Cmd {
	return func() tea.Msg {
		_ = m.client.Send(cmd, payload)
		return nil
	}
}

func (m model) submitCredential(stage, value string) tea.Cmd {
	return m.sendCmd(daemon.CmdSubmitCredential, daemon.SubmitCredentialPayload{
		Stage:    stage,
		Value:    value,
		Remember: m.rememberCreds,
	})
}

func appendSample(buf []float64, v float64, max int) []float64 {
	buf = append(buf, v)
	if len(buf) > max {
		buf = buf[len(buf)-max:]
	}
	return buf
}

func appendLog(buf []string, line string, max int) []string {
	buf = append(buf, line)
	if len(buf) > max {
		buf = buf[len(buf)-max:]
	}
	return buf
}

// --- View -------------------------------------------------------------------

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Inicializando..."
	}

	// Si hay prompt activo, ocupamos toda la pantalla con el popup
	// centrado — se siente como modal de verdad en vez de un cuadro
	// colado abajo del dashboard.
	if m.prompt != nil {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderPrompt(),
		)
	}

	// Ancho del contenido: mínimo 60, máximo 120, adaptativo a la terminal
	// con un margen lateral de 4 columnas.
	contentWidth := m.width - 4
	if contentWidth < 60 {
		contentWidth = m.width
	}
	if contentWidth > 120 {
		contentWidth = 120
	}

	parts := []string{
		m.renderHeader(contentWidth),
		m.renderTabs(contentWidth),
	}
	if m.active == tabDashboard {
		parts = append(parts, m.renderDashboard(contentWidth))
	} else {
		parts = append(parts, m.renderLogs(contentWidth))
	}
	parts = append(parts, m.renderHelp(contentWidth))

	body := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if m.err != "" {
		banner := errStyle.Render("✗ " + m.err)
		body = lipgloss.JoinVertical(lipgloss.Left, body, banner)
	}

	// Centrar horizontalmente todo el dashboard en la terminal.
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, body)
}

func (m model) renderHeader(w int) string {
	brand := cyanStyle.Render("NAV") + brandAccent.Render("TUNNEL")
	left := lipgloss.JoinHorizontal(lipgloss.Center,
		cyanStyle.Render("◈ "),
		brand,
		dimStyle.Render("  v"+m.version),
	)
	right := lipgloss.JoinHorizontal(lipgloss.Center,
		dotForState(m.state), " ",
		valueStyle.Render(m.state),
	)
	line := lipgloss.PlaceHorizontal(w, lipgloss.Left, left) // trick: relleno
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line = left + strings.Repeat(" ", gap) + right
	bar := dimStyle.Render(strings.Repeat("─", w))
	return lipgloss.JoinVertical(lipgloss.Left, line, bar)
}

func (m model) renderTabs(w int) string {
	tabs := []string{"DASHBOARD", "LOGS"}
	items := make([]string, len(tabs))
	for i, t := range tabs {
		if int(m.active) == i {
			items[i] = tabActive.Render(t)
		} else {
			items[i] = tabInactive.Render(t)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, items...)
	bar := dimStyle.Render(strings.Repeat("─", w))
	return lipgloss.JoinVertical(lipgloss.Left, row, bar)
}

func (m model) renderDashboard(w int) string {
	chartHeight := 1
	chartWidth := w - 6
	if chartWidth < 20 {
		chartWidth = 20
	}
	_ = chartHeight
	rxSpark := Sparkline(m.rxBuf, chartWidth, cyanStyle)
	txSpark := Sparkline(m.txBuf, chartWidth, magentaStyle)

	chartCard := panelStyle.Width(w - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			dimTitleStyle.Render("TRÁFICO EN VIVO"),
			"",
			cyanStyle.Render("↓ RX ")+dimStyle.Render(" ")+rxSpark,
			dimStyle.Render("     "+fmt.Sprintf("%-12s  %s", HumanBytes(m.bytesIn), HumanRate(m.rateIn))),
			"",
			magentaStyle.Render("↑ TX ")+dimStyle.Render(" ")+txSpark,
			dimStyle.Render("     "+fmt.Sprintf("%-12s  %s", HumanBytes(m.bytesOut), HumanRate(m.rateOut))),
		),
	)

	tun := m.tunIP
	if tun == "" {
		tun = "—"
	}
	rem := m.remoteIP
	if rem == "" {
		rem = "—"
	}
	ovpn := m.ovpn
	if ovpn == "" {
		ovpn = "— (configurá con navtunnel GUI, o próximamente desde la TUI)"
	}

	infoCard := panelStyle.Width(w - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			dimTitleStyle.Render("CONEXIÓN"),
			"",
			dimStyle.Render("TUN      ")+valueStyle.Render(tun),
			dimStyle.Render("SERVIDOR ")+valueStyle.Render(rem),
			dimStyle.Render("CONFIG   ")+valueStyle.Render(ovpn),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left, chartCard, infoCard)
}

func (m model) renderLogs(w int) string {
	// Últimas N líneas según alto disponible.
	maxLines := m.height - 10
	if maxLines < 6 {
		maxLines = 6
	}
	lines := m.logs
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	rendered := make([]string, len(lines))
	for i, l := range lines {
		rendered[i] = logLineStyle.Render(l)
	}
	body := strings.Join(rendered, "\n")
	return panelStyle.Width(w - 2).Height(maxLines + 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			dimTitleStyle.Render("LOG STREAM"),
			"",
			body,
		),
	)
}

func (m model) renderHelp(w int) string {
	if m.prompt != nil {
		return helpStyle.Render("enter: enviar   esc: cancelar   ctrl+r: recordar [" + checkbox(m.rememberCreds) + "]")
	}
	keys := []string{
		cyanStyle.Render("c") + dimStyle.Render(" conectar"),
		cyanStyle.Render("d") + dimStyle.Render(" desconectar"),
		cyanStyle.Render("tab") + dimStyle.Render(" cambiar tab"),
		cyanStyle.Render("s") + dimStyle.Render(" apagar daemon"),
		cyanStyle.Render("q") + dimStyle.Render(" salir (el daemon sigue)"),
	}
	return helpStyle.Render(strings.Join(keys, "  "))
}

func checkbox(b bool) string {
	if b {
		return "✓"
	}
	return " "
}

// renderPrompt dibuja el modal de autenticación como una única caja
// pensada para ocupar el centro de la pantalla. La envoltura de
// lipgloss.Place() en View() la posiciona horizontal+verticalmente.
func (m model) renderPrompt() string {
	title := strings.ToUpper(m.prompt.stage)
	switch m.prompt.stage {
	case "user":
		title = "USUARIO"
	case "pass":
		title = "CONTRASEÑA"
	case "otp":
		title = "CÓDIGO OTP"
	}

	// El width del popup crece con la terminal pero queda contenido.
	boxWidth := 56
	if m.width < 60 {
		boxWidth = m.width - 4
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	help := helpStyle.Render("enter: enviar · esc: cancelar · ctrl+r: recordar [" + checkbox(m.rememberCreds) + "]")

	// En el OTP no se ofrece "recordar" — es single-use por definición.
	if m.prompt.stage == "otp" {
		help = helpStyle.Render("enter: enviar · esc: cancelar")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		promptTitle.Render("◈ "+title),
		"",
		dimStyle.Render(m.prompt.message),
		"",
		m.input.View(),
		"",
		help,
	)
	return promptBox.Width(boxWidth).Render(content)
}
