package core

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lavp2393/navtunnel/internal/platform"
)

// authReadTimeout es el deadline por operación de lectura durante el
// handshake inicial con el management. Se aplica a cada Read por separado,
// no como deadline total, para no acumular tiempo entre líneas.
const authReadTimeout = 10 * time.Second

// EventType representa el tipo de evento emitido por el Manager.
type EventType int

const (
	EventAskUser EventType = iota
	EventAskPass
	EventAskOTP
	EventConnected
	EventAuthFailed
	EventFatal
	EventLogLine
	EventDisconnected
	EventState     // Nuevo: cambios de estado de OpenVPN con IPs.
	EventBytecount // Nuevo: contadores de tráfico acumulados.
)

// Event describe un suceso en la sesión OpenVPN.
// Los campos opcionales se populan según Type.
type Event struct {
	Type    EventType
	Message string
	Stage   string // Para EventAuthFailed: "username" | "password" | "otp".

	// EventState:
	State      string
	LocalTunIP string
	RemoteIP   string

	// EventBytecount:
	BytesIn  uint64
	BytesOut uint64
}

// SendFns agrupa los callbacks que la UI invoca para entregar credenciales.
type SendFns struct {
	Username func(string) error
	Password func(string) error
	OTP      func(string) error
}

type credState int

const (
	csIdle credState = iota
	csWaitUser
	csWaitPass
	csWaitOTPStatic  // Pendiente un OTP para combinar con pass (SCRV1).
	csWaitOTPDynamic // Pendiente un OTP ante challenge dinámico (CRV1) tras fallo.
	csSent
)

// Manager controla el ciclo de vida de OpenVPN y habla con su management
// interface por socket TCP local autenticado por cookie.
type Manager struct {
	cmd    *exec.Cmd
	conn   net.Conn
	reader *bufio.Reader

	events     chan Event
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
	procExited chan struct{} // se cierra cuando cmd.Wait() retorna

	writeMu sync.Mutex // Serializa escrituras al socket.

	credMu              sync.Mutex
	credStateVal        credState
	pendingUser         string
	pendingPass         string
	hasStaticChallenge  bool
	staticChallengeText string
	lastCRV1State       string

	pwFilePath string // Archivo temporal con la cookie, se borra en Stop().
	metrics    *metricsStore
}

// Start lanza OpenVPN con management activo y se conecta. El ovpnPath es el
// archivo .ovpn del usuario; openvpnBinary es opcional (se busca en PATH si
// está vacío).
func Start(ovpnPath, openvpnBinary string) (*Manager, error) {
	if openvpnBinary == "" {
		var err error
		openvpnBinary, err = FindOpenVPN()
		if err != nil {
			return nil, err
		}
	}

	port, err := findFreePort()
	if err != nil {
		return nil, fmt.Errorf("no se encontró puerto libre para management: %w", err)
	}

	cookie, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	pwFile, err := writePasswordFile(cookie)
	if err != nil {
		return nil, err
	}

	args := []string{
		"--config", ovpnPath,
		"--management", "127.0.0.1", strconv.Itoa(port), pwFile,
		"--management-query-passwords",
		"--management-hold",
		"--auth-retry", "interact",
		"--auth-nocache",
		"--verb", "3",
	}

	plat := platform.New()
	program, fullArgs, err := plat.ElevateCommand(openvpnBinary, args)
	if err != nil {
		_ = os.Remove(pwFile)
		return nil, fmt.Errorf("preparando elevación: %w", err)
	}

	cmd := exec.Command(program, fullArgs...)
	// Capturamos stdout/stderr además del management socket: antes de que el
	// management esté listo (arranque temprano, errores de config, mensajes
	// del elevador pkexec/osascript), el socket aún no emite, pero stderr sí.
	// Los volcamos a m.events como EventLogLine.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.Remove(pwFile)
		return nil, fmt.Errorf("creando stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		_ = os.Remove(pwFile)
		return nil, fmt.Errorf("creando stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(pwFile)
		return nil, fmt.Errorf("iniciando openvpn: %w", err)
	}

	conn, err := dialManagement("127.0.0.1:"+strconv.Itoa(port), 8*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = os.Remove(pwFile)
		return nil, fmt.Errorf("conectando al management socket: %w", err)
	}

	m := &Manager{
		cmd:        cmd,
		conn:       conn,
		reader:     bufio.NewReader(conn),
		events:     make(chan Event, 256),
		stopCh:     make(chan struct{}),
		procExited: make(chan struct{}),
		pwFilePath: pwFile,
		metrics:    newMetricsStore(),
	}

	// Drenar stdout/stderr desde ya para que cualquier mensaje temprano del
	// proceso (p.ej. "Options error: ..." antes de abrir el management)
	// aparezca en la UI sin bloquear al proceso por pipes llenos.
	m.wg.Add(2)
	go m.drainPipe(stdoutPipe, "[openvpn] ")
	go m.drainPipe(stderrPipe, "[openvpn] ")

	if err := m.authenticateSocket(cookie); err != nil {
		m.cleanupStart()
		return nil, err
	}

	// Habilitar eventos asíncronos y liberar el hold.
	for _, c := range []string{"state on", "bytecount 1", "log on all", "hold release"} {
		if err := m.writeCommand(c); err != nil {
			m.cleanupStart()
			return nil, fmt.Errorf("enviando %q al management: %w", c, err)
		}
	}

	m.wg.Add(2)
	go m.readLoop()
	go m.waitProcess()

	return m, nil
}

// drainPipe lee línea por línea del stdout/stderr del proceso openvpn y
// reemite cada línea como EventLogLine con un prefijo discriminante.
func (m *Manager) drainPipe(r io.ReadCloser, prefix string) {
	defer m.wg.Done()
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 1024*1024)
	for sc.Scan() {
		m.emit(Event{Type: EventLogLine, Message: prefix + sc.Text()})
	}
}

// Events expone el canal de eventos del manager.
func (m *Manager) Events() <-chan Event { return m.events }

// Metrics devuelve una copia del estado actual de métricas.
func (m *Manager) Metrics() Metrics { return m.metrics.Snapshot() }

// SendFunctions devuelve los callbacks para entregar credenciales.
// La API es estable: la UI puede llamarlos en el orden Username → Password → OTP.
func (m *Manager) SendFunctions() SendFns {
	return SendFns{
		Username: m.sendUsername,
		Password: m.sendPassword,
		OTP:      m.sendOTP,
	}
}

// Stop termina el proceso OpenVPN y cierra el manager. Es idempotente.
// Intenta cierre limpio vía "signal SIGTERM" por el management socket — esto
// es lo único que funciona en macOS, donde matar el proceso padre (osascript)
// no propaga la señal al openvpn elevado.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)

		// Pedir shutdown limpio a openvpn. Si el socket ya está cerrado, sigue.
		if m.conn != nil {
			_ = m.writeCommandIgnoreStop("signal SIGTERM")
		}

		// Esperar a que el proceso salga solo; si no lo hace en 1.5 s, forzar.
		select {
		case <-m.procExited:
		case <-time.After(1500 * time.Millisecond):
			if m.cmd != nil && m.cmd.Process != nil {
				_ = m.cmd.Process.Kill()
			}
		}

		if m.conn != nil {
			_ = m.conn.Close()
		}
		if m.pwFilePath != "" {
			_ = os.Remove(m.pwFilePath)
		}
		m.wg.Wait()
		close(m.events)
	})
}

// writeCommandIgnoreStop escribe sin revisar stopCh. Solo lo usamos durante
// Stop() para enviar "signal SIGTERM" justo después de cerrar stopCh.
func (m *Manager) writeCommandIgnoreStop(cmd string) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	if m.conn == nil {
		return fmt.Errorf("socket no disponible")
	}
	_, err := m.conn.Write([]byte(cmd + "\n"))
	return err
}

// --- Autenticación del socket management ------------------------------------

// authenticateSocket completa el handshake con el management:
//  1. Espera ver "ENTER PASSWORD:" — openvpn lo emite sin newline, así que
//     leemos caracter a caracter y cortamos al detectarlo.
//  2. Envía el cookie + "\n".
//  3. Espera una línea que empiece con "SUCCESS:" (o "ERROR:" si el cookie
//     no coincide, cosa que no debería pasar porque la generamos nosotros).
//
// El deadline se renueva antes de cada operación de lectura para no
// acumular el tiempo entre pasos del handshake.
func (m *Manager) authenticateSocket(cookie string) error {
	defer func() { _ = m.conn.SetReadDeadline(time.Time{}) }()

	if err := m.readUntilPasswordPrompt(); err != nil {
		return fmt.Errorf("management no envió prompt de auth: %w", err)
	}
	if _, err := m.conn.Write([]byte(cookie + "\n")); err != nil {
		return fmt.Errorf("escribiendo cookie de auth: %w", err)
	}
	_ = m.conn.SetReadDeadline(time.Now().Add(authReadTimeout))
	line, err := m.reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("leyendo respuesta de auth: %w", err)
	}
	resp := strings.TrimSpace(line)
	if !strings.HasPrefix(resp, "SUCCESS:") {
		return fmt.Errorf("auth al management falló: %s", resp)
	}
	return nil
}

// readUntilPasswordPrompt consume el socket byte a byte hasta que el buffer
// acumulado contenga "ENTER PASSWORD:". openvpn emite ese prompt sin
// newline, así que ReadString('\n') bloquearía. Deja en el buffer cualquier
// dato posterior (el siguiente Read ya es bufio).
func (m *Manager) readUntilPasswordPrompt() error {
	const needle = "ENTER PASSWORD:"
	var acc strings.Builder
	for {
		_ = m.conn.SetReadDeadline(time.Now().Add(authReadTimeout))
		b, err := m.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("socket cerrado sin prompt (leído: %q)", acc.String())
			}
			return err
		}
		acc.WriteByte(b)
		if strings.Contains(acc.String(), needle) {
			return nil
		}
		// Evitar que acc crezca indefinidamente si algo raro ocurre.
		if acc.Len() > 4096 {
			return fmt.Errorf("no se encontró prompt tras 4KB: %q", acc.String())
		}
	}
}

// --- Loop principal ---------------------------------------------------------

func (m *Manager) readLoop() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}
		line, err := m.reader.ReadString('\n')
		if line != "" {
			m.handleLine(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

func (m *Manager) handleLine(line string) {
	if line == "" {
		return
	}
	switch {
	case strings.HasPrefix(line, ">STATE:"):
		m.handleState(line)
	case strings.HasPrefix(line, ">BYTECOUNT:"):
		m.handleBytecount(line)
	case strings.HasPrefix(line, ">PASSWORD:"):
		m.handlePassword(line)
	case strings.HasPrefix(line, ">LOG:"):
		m.emit(Event{Type: EventLogLine, Message: stripLogPrefix(line)})
	case strings.HasPrefix(line, ">HOLD:"):
		// El hold inicial ya fue liberado en Start; si vuelve a aparecer (reintento),
		// volvemos a liberar.
		_ = m.writeCommand("hold release")
	case strings.HasPrefix(line, ">FATAL:"):
		m.emit(Event{Type: EventFatal, Message: strings.TrimPrefix(line, ">FATAL:")})
	case strings.HasPrefix(line, ">INFO:"):
		// Banner inicial, lo tratamos como log normal.
		m.emit(Event{Type: EventLogLine, Message: strings.TrimPrefix(line, ">INFO:")})
	case strings.HasPrefix(line, "SUCCESS:"), strings.HasPrefix(line, "ERROR:"), line == "END":
		// Respuestas a comandos; no son eventos para la UI.
	default:
		// Línea no reconocida: la exponemos como log para no perderla.
		m.emit(Event{Type: EventLogLine, Message: line})
	}
}

func (m *Manager) handleState(line string) {
	name, localTun, remote, ok := stateFromLine(line)
	if !ok {
		return
	}
	m.metrics.applyState(name, localTun, remote)
	m.emit(Event{
		Type:       EventState,
		Message:    name,
		State:      name,
		LocalTunIP: localTun,
		RemoteIP:   remote,
	})
	switch name {
	case "CONNECTED":
		m.emit(Event{Type: EventConnected, Message: "Conexión establecida"})
	case "EXITING":
		m.emit(Event{Type: EventDisconnected, Message: "OpenVPN saliendo"})
	}
}

func (m *Manager) handleBytecount(line string) {
	in, out, ok := bytecountFromLine(line)
	if !ok {
		return
	}
	m.metrics.applyBytecount(in, out)
	m.emit(Event{Type: EventBytecount, BytesIn: in, BytesOut: out})
}

func (m *Manager) handlePassword(line string) {
	p := parsePasswordPrompt(line)
	if p == nil {
		return
	}
	// Solo soportamos el realm Auth (credenciales corporativas).
	if p.realm != "" && p.realm != "Auth" {
		m.emit(Event{
			Type:    EventFatal,
			Message: fmt.Sprintf("realm no soportado: %q (ej. certificado con passphrase)", p.realm),
		})
		return
	}

	m.credMu.Lock()
	if p.crv1 {
		m.credStateVal = csWaitOTPDynamic
		m.lastCRV1State = p.crv1State
		msg := p.crv1Text
		if msg == "" {
			msg = "Ingresa tu código OTP"
		}
		m.credMu.Unlock()
		m.emit(Event{Type: EventAskOTP, Message: msg})
		return
	}

	if p.needsCredential {
		m.hasStaticChallenge = p.staticChallenge != ""
		m.staticChallengeText = p.staticChallenge
		m.credStateVal = csWaitUser
		m.credMu.Unlock()
		m.emit(Event{Type: EventAskUser, Message: "Ingresa tu usuario corporativo"})
		return
	}

	// Verification Failed sin CRV1 ni Need: auth rechazada "pura".
	// Reportamos usando la etapa donde estábamos esperando respuesta.
	stage := credStageFor(m.credStateVal)
	m.credMu.Unlock()
	m.emit(Event{Type: EventAuthFailed, Message: "Autenticación rechazada", Stage: stage})
}

// credStageFor mapea el estado interno a la etapa lógica que usa la UI
// para mensajes de AUTH_FAILED ("username" | "password" | "otp").
func credStageFor(s credState) string {
	switch s {
	case csWaitUser:
		return "username"
	case csWaitPass, csSent:
		return "password"
	case csWaitOTPStatic, csWaitOTPDynamic:
		return "otp"
	default:
		return "password"
	}
}

// --- Envío de credenciales --------------------------------------------------

func (m *Manager) sendUsername(user string) error {
	if err := validateCredInput(user); err != nil {
		return err
	}
	m.credMu.Lock()
	m.pendingUser = user
	m.credStateVal = csWaitPass
	m.credMu.Unlock()

	m.emit(Event{Type: EventAskPass, Message: "Ingresa tu contraseña"})
	return nil
}

func (m *Manager) sendPassword(pass string) error {
	if err := validateCredInput(pass); err != nil {
		return err
	}
	m.credMu.Lock()
	m.pendingPass = pass
	if m.hasStaticChallenge {
		msg := m.staticChallengeText
		m.credStateVal = csWaitOTPStatic
		m.credMu.Unlock()
		if msg == "" {
			msg = "Ingresa tu código OTP"
		}
		m.emit(Event{Type: EventAskOTP, Message: msg})
		return nil
	}
	user := m.pendingUser
	m.credStateVal = csSent
	// No limpiamos pendingPass aún; si el server responde con CRV1 dinámico
	// el siguiente sendOTP no lo necesita, pero lo borramos en Stop() o al reiniciar flujo.
	m.credMu.Unlock()

	return m.writeAuthPair(user, pass)
}

func (m *Manager) sendOTP(otp string) error {
	if err := validateCredInput(otp); err != nil {
		return err
	}
	m.credMu.Lock()
	state := m.credStateVal
	user := m.pendingUser
	pass := m.pendingPass
	crv := m.lastCRV1State
	m.credStateVal = csSent
	m.credMu.Unlock()

	var passField string
	switch state {
	case csWaitOTPStatic:
		passField = buildStaticChallengeResponse(pass, otp)
	case csWaitOTPDynamic:
		passField = buildDynamicChallengeResponse(crv, otp)
	default:
		return fmt.Errorf("no se esperaba un OTP en el estado actual")
	}
	return m.writeAuthPair(user, passField)
}

func (m *Manager) writeAuthPair(user, passField string) error {
	if err := m.writeCommand("username " + encodeCommandArg("Auth") + " " + encodeCommandArg(user)); err != nil {
		return err
	}
	return m.writeCommand("password " + encodeCommandArg("Auth") + " " + encodeCommandArg(passField))
}

func validateCredInput(s string) error {
	if strings.ContainsAny(s, "\n\r\x00") {
		return fmt.Errorf("credencial contiene caracteres de control no permitidos")
	}
	return nil
}

// --- Gestión del subproceso -------------------------------------------------

func (m *Manager) waitProcess() {
	defer m.wg.Done()
	defer close(m.procExited)

	err := m.cmd.Wait()
	msg := "Proceso OpenVPN terminado"
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		msg = fmt.Sprintf("OpenVPN terminó: %v", err)
	}
	m.emit(Event{Type: EventDisconnected, Message: msg})
}

// --- Utilidades del socket --------------------------------------------------

func (m *Manager) writeCommand(cmd string) error {
	select {
	case <-m.stopCh:
		return fmt.Errorf("manager detenido")
	default:
	}
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	if m.conn == nil {
		return fmt.Errorf("socket no disponible")
	}
	_, err := m.conn.Write([]byte(cmd + "\n"))
	return err
}

func (m *Manager) emit(e Event) {
	defer func() { _ = recover() }()
	select {
	case m.events <- e:
	case <-m.stopCh:
	}
}

func (m *Manager) cleanupStart() {
	if m.conn != nil {
		_ = m.conn.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	// Esperar que las goroutines de drain (stdout/stderr) salgan tras el Kill.
	// Normalmente toma <100 ms (el kernel cierra los FDs y Scan() retorna).
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	_ = os.Remove(m.pwFilePath)
}

// --- Helpers auxiliares -----------------------------------------------------

// findFreePort pide al kernel un puerto libre en loopback y lo devuelve.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// randomHex devuelve n bytes aleatorios codificados como hex string.
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// writePasswordFile escribe la cookie a un archivo temporal con permisos 0600
// y devuelve su path. El archivo se borra en Stop().
func writePasswordFile(cookie string) (string, error) {
	f, err := os.CreateTemp("", "navtunnel-mgmt-*.pw")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := f.WriteString(cookie + "\n"); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// dialManagement intenta conectar al socket management con reintentos
// exponenciales dentro del timeout total. openvpn demora decenas a cientos
// de ms en levantar el listener después de arrancar.
func dialManagement(addr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	wait := 50 * time.Millisecond
	var lastErr error
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			return c, nil
		}
		lastErr = err
		time.Sleep(wait)
		if wait < 400*time.Millisecond {
			wait *= 2
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return nil, lastErr
}

func stripLogPrefix(line string) string {
	// >LOG:timestamp,flags,message → nos quedamos con el mensaje para la UI.
	payload := strings.TrimPrefix(line, ">LOG:")
	fields := strings.SplitN(payload, ",", 3)
	if len(fields) < 3 {
		return payload
	}
	return fields[2]
}

