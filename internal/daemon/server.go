package daemon

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lavp2393/navtunnel/internal/config"
	"github.com/lavp2393/navtunnel/internal/core"
	"github.com/lavp2393/navtunnel/internal/logs"
)

// Server es el daemon que mantiene el Manager de OpenVPN vivo entre
// ejecuciones del cliente. Corre en foreground (el proceso hijo ya viene
// forkeado/setsid desde el cliente) hasta que recibe shutdown o SIGTERM.
type Server struct {
	listener net.Listener
	token    string
	version  string

	cfg    *config.Config
	logBuf *logs.Buffer

	mu      sync.Mutex
	manager *core.Manager
	sendFns core.SendFns

	credMu        sync.Mutex
	savedUser     string
	savedPass     string
	rememberCreds bool

	// clientes conectados.
	clientsMu sync.Mutex
	clients   map[*client]struct{}

	// cached metrics snapshot, refrescado desde los eventos del Manager.
	metricsMu sync.Mutex
	metrics   StatePayload

	shutdownCh chan struct{}
	stopOnce   sync.Once
}

// NewServer construye el daemon pero no arranca a escuchar hasta Serve().
func NewServer(version string) (*Server, error) {
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
		log.Printf("daemon: config load: %v", err)
	}
	if cfg == nil {
		cfg = config.Default()
	}

	s := &Server{
		version:    version,
		cfg:        cfg,
		logBuf:     logs.NewBuffer(400),
		clients:    make(map[*client]struct{}),
		shutdownCh: make(chan struct{}),
	}
	s.loadStoredCredentials()
	return s, nil
}

// Serve bindea un puerto TCP libre en loopback, publica el InfoFile con el
// token generado y acepta clientes hasta que alguno pida shutdown.
func (s *Server) Serve() error {
	// Singleton: si ya hay otro daemon dueño del lock, rendirnos antes de
	// bindear un puerto. Mantenemos el FD abierto durante toda la vida.
	lockFD, err := acquireSingleton()
	if err != nil {
		return err
	}
	defer lockFD.Close()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = l
	port := l.Addr().(*net.TCPAddr).Port

	token, err := randomHex(16)
	if err != nil {
		return err
	}
	s.token = token

	if _, err := writeInfo(InfoFile{Port: port, Token: token, PID: os.Getpid()}); err != nil {
		return fmt.Errorf("write info: %w", err)
	}
	defer removeInfo()

	// Si una sesión anterior dejó un openvpn huérfano (navtunnel-cli
	// asesinado antes de desconectar limpio), lo matamos acá. Sólo tocamos
	// procesos con nuestra firma (--management-hold + pw-file prefix
	// "navtunnel-mgmt-"); cualquier otro openvpn queda intacto.
	if n := killOrphanOpenVPN(); n > 0 {
		s.log(fmt.Sprintf("limpiados %d proceso(s) openvpn huérfano(s) de sesión previa", n))
	}

	s.log(fmt.Sprintf("daemon listening on 127.0.0.1:%d", port))

	go s.acceptLoop()

	<-s.shutdownCh
	_ = s.listener.Close()
	s.disconnect()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownCh:
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.log("accept error: " + err.Error())
			continue
		}
		go s.serveClient(conn)
	}
}

// --- Cliente ----------------------------------------------------------------

type client struct {
	conn   net.Conn
	enc    *json.Encoder
	writeM sync.Mutex
}

func (c *client) send(typ string, payload any) error {
	env := Envelope{Type: typ}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		env.Payload = data
	}
	c.writeM.Lock()
	defer c.writeM.Unlock()
	return c.enc.Encode(&env)
}

func (s *Server) serveClient(conn net.Conn) {
	defer conn.Close()

	// Handshake: el cliente manda una línea con el token.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	tokenIn := []byte(line[:len(line)-1])
	if len(tokenIn) > 0 && tokenIn[len(tokenIn)-1] == '\r' {
		tokenIn = tokenIn[:len(tokenIn)-1]
	}
	if subtle.ConstantTimeCompare(tokenIn, []byte(s.token)) != 1 {
		_, _ = conn.Write([]byte(`{"type":"reply","payload":{"ok":false,"error":"auth"}}` + "\n"))
		return
	}

	c := &client{conn: conn, enc: json.NewEncoder(conn)}
	s.clientsMu.Lock()
	s.clients[c] = struct{}{}
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, c)
		s.clientsMu.Unlock()
	}()

	// Hello + snapshot inicial.
	_ = c.send(EvtHello, HelloPayload{Version: s.version, OvpnPath: s.cfg.VPNConfigPath})
	_ = c.send(EvtState, s.snapshotMetrics())
	for _, l := range s.logBuf.GetAll() {
		_ = c.send(EvtLog, LogPayload{Line: l})
	}

	// Loop: leer comandos del cliente.
	dec := json.NewDecoder(br)
	for {
		var cmd CommandEnvelope
		if err := dec.Decode(&cmd); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		s.dispatch(c, cmd)
	}
}

// dispatch procesa un comando del cliente.
func (s *Server) dispatch(c *client, cmd CommandEnvelope) {
	reply := func(ok bool, errMsg string) {
		_ = c.send(EvtReply, ReplyPayload{ID: cmd.ID, OK: ok, Error: errMsg})
	}

	switch cmd.Type {
	case CmdPing:
		reply(true, "")

	case CmdSubscribe:
		// El cliente ya está suscripto por default en serveClient; este comando
		// se mantiene por simetría futura.
		reply(true, "")

	case CmdConnect:
		if err := s.connect(); err != nil {
			reply(false, err.Error())
			return
		}
		reply(true, "")

	case CmdDisconnect:
		go s.disconnect()
		reply(true, "")

	case CmdSubmitCredential:
		var p SubmitCredentialPayload
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			reply(false, "invalid payload: "+err.Error())
			return
		}
		if err := s.submitCredential(p); err != nil {
			reply(false, err.Error())
			return
		}
		reply(true, "")

	case CmdSaveConfig:
		var p SaveConfigPayload
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			reply(false, "invalid payload: "+err.Error())
			return
		}
		if err := s.saveConfig(p.OvpnPath); err != nil {
			reply(false, err.Error())
			return
		}
		reply(true, "")

	case CmdShutdown:
		reply(true, "")
		go s.Shutdown()

	default:
		reply(false, "unknown command: "+cmd.Type)
	}
}

// Shutdown solicita el cierre ordenado del daemon.
func (s *Server) Shutdown() {
	s.stopOnce.Do(func() { close(s.shutdownCh) })
}

// ShutdownCh expone el canal para que main lo observe (p.ej. wait SIGTERM).
func (s *Server) ShutdownCh() <-chan struct{} { return s.shutdownCh }

// --- Manager lifecycle ------------------------------------------------------

func (s *Server) connect() error {
	if !s.cfg.IsVPNConfigValid() {
		return errors.New("archivo .ovpn no configurado o inaccesible")
	}
	s.mu.Lock()
	if s.manager != nil {
		s.mu.Unlock()
		return errors.New("ya hay una conexión activa")
	}
	s.mu.Unlock()

	openvpnPath, err := core.FindOpenVPN()
	if err != nil {
		return err
	}
	s.log("Usando openvpn: " + openvpnPath)

	mgr, err := core.Start(s.cfg.VPNConfigPath, openvpnPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.manager = mgr
	s.sendFns = mgr.SendFunctions()
	s.mu.Unlock()

	go s.pumpEvents(mgr)
	return nil
}

func (s *Server) disconnect() {
	s.mu.Lock()
	mgr := s.manager
	s.manager = nil
	s.sendFns = core.SendFns{}
	s.mu.Unlock()
	if mgr != nil {
		mgr.Stop()
	}
}

// pumpEvents traduce los core.Events en broadcasts al hub de clientes.
func (s *Server) pumpEvents(mgr *core.Manager) {
	for ev := range mgr.Events() {
		s.routeEvent(ev)
	}
	// Canal cerrado: limpiar state.
	s.mu.Lock()
	if s.manager == mgr {
		s.manager = nil
		s.sendFns = core.SendFns{}
	}
	s.mu.Unlock()
	s.updateMetricsState("DISCONNECTED", "", "")
	s.broadcast(EvtDisconnect, DisconnectedPayload{Reason: "manager closed"})
}

func (s *Server) routeEvent(ev core.Event) {
	switch ev.Type {
	case core.EventLogLine:
		s.log(ev.Message)

	case core.EventState:
		s.updateMetricsState(ev.State, ev.LocalTunIP, ev.RemoteIP)
		s.broadcast(EvtState, s.snapshotMetrics())

	case core.EventBytecount:
		rateIn, rateOut := s.updateMetricsBytes(ev.BytesIn, ev.BytesOut)
		s.broadcast(EvtBytecount, BytecountPayload{
			BytesIn: ev.BytesIn, BytesOut: ev.BytesOut,
			RateIn: rateIn, RateOut: rateOut,
		})

	case core.EventAskUser:
		s.credMu.Lock()
		saved := s.savedUser
		s.credMu.Unlock()
		if saved != "" && s.autoSendUsername(saved) {
			return
		}
		s.broadcast(EvtAskUser, AskPayload{Message: ev.Message})

	case core.EventAskPass:
		s.credMu.Lock()
		saved := s.savedPass
		s.credMu.Unlock()
		if saved != "" && s.autoSendPassword(saved) {
			return
		}
		s.broadcast(EvtAskPass, AskPayload{Message: ev.Message})

	case core.EventAskOTP:
		s.broadcast(EvtAskOTP, AskPayload{Message: ev.Message})

	case core.EventConnected:
		s.log(ev.Message)
		s.broadcast(EvtConnected, nil)

	case core.EventAuthFailed:
		s.invalidateSavedFor(ev.Stage)
		s.broadcast(EvtAuthFailed, AuthFailedPayload{Stage: ev.Stage, Message: ev.Message})

	case core.EventFatal:
		s.log("FATAL: " + ev.Message)
		s.broadcast(EvtFatal, FatalPayload{Message: ev.Message})

	case core.EventDisconnected:
		s.log(ev.Message)
		s.broadcast(EvtDisconnect, DisconnectedPayload{Reason: ev.Message})
	}
}

// --- Métricas ---------------------------------------------------------------

func (s *Server) snapshotMetrics() StatePayload {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	cpy := s.metrics
	if cpy.State == "" {
		cpy.State = "DISCONNECTED"
	}
	return cpy
}

func (s *Server) updateMetricsState(state, tun, remote string) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.State = state
	if tun != "" {
		s.metrics.LocalTunIP = tun
	}
	if remote != "" {
		s.metrics.RemoteServerIP = remote
	}
}

// updateMetricsBytes actualiza los totales y recalcula las tasas desde el
// snapshot del Manager; devuelve (rateIn, rateOut).
func (s *Server) updateMetricsBytes(in, out uint64) (uint64, uint64) {
	s.mu.Lock()
	mgr := s.manager
	s.mu.Unlock()
	var rateIn, rateOut uint64
	if mgr != nil {
		m := mgr.Metrics()
		rateIn, rateOut = m.BytesInRate, m.BytesOutRate
	}
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.BytesIn = in
	s.metrics.BytesOut = out
	s.metrics.BytesInRate = rateIn
	s.metrics.BytesOutRate = rateOut
	return rateIn, rateOut
}

// --- Broadcast --------------------------------------------------------------

func (s *Server) broadcast(typ string, payload any) {
	s.clientsMu.Lock()
	targets := make([]*client, 0, len(s.clients))
	for c := range s.clients {
		targets = append(targets, c)
	}
	s.clientsMu.Unlock()
	for _, c := range targets {
		_ = c.send(typ, payload)
	}
}

func (s *Server) log(line string) {
	s.logBuf.Add(line)
	s.broadcast(EvtLog, LogPayload{Line: line})
}

// --- Credenciales guardadas -------------------------------------------------

func (s *Server) loadStoredCredentials() {
	user, pass, _, _, err := core.LoadCredentials()
	if err != nil {
		return
	}
	s.credMu.Lock()
	s.savedUser = user
	s.savedPass = pass
	s.rememberCreds = true
	s.credMu.Unlock()
}

func (s *Server) submitCredential(p SubmitCredentialPayload) error {
	s.mu.Lock()
	fns := s.sendFns
	s.mu.Unlock()

	switch p.Stage {
	case "user":
		s.credMu.Lock()
		s.savedUser = p.Value
		s.rememberCreds = p.Remember
		s.credMu.Unlock()
		if fns.Username == nil {
			return errors.New("manager no preparado")
		}
		return fns.Username(p.Value)

	case "pass":
		s.credMu.Lock()
		s.savedPass = p.Value
		s.rememberCreds = p.Remember
		user := s.savedUser
		s.credMu.Unlock()
		s.persistCredentials(user, p.Value, p.Remember)
		if fns.Password == nil {
			return errors.New("manager no preparado")
		}
		return fns.Password(p.Value)

	case "otp":
		if fns.OTP == nil {
			return errors.New("manager no preparado")
		}
		return fns.OTP(p.Value)
	}
	return fmt.Errorf("etapa desconocida: %q", p.Stage)
}

func (s *Server) autoSendUsername(v string) bool {
	s.mu.Lock()
	fn := s.sendFns.Username
	s.mu.Unlock()
	if fn == nil {
		return false
	}
	if err := fn(v); err != nil {
		s.log("No se pudo enviar usuario guardado: " + err.Error())
		return false
	}
	s.log("✓ Usuario enviado (recordado)")
	return true
}

func (s *Server) autoSendPassword(v string) bool {
	s.mu.Lock()
	fn := s.sendFns.Password
	s.mu.Unlock()
	if fn == nil {
		return false
	}
	if err := fn(v); err != nil {
		s.log("No se pudo enviar contraseña guardada: " + err.Error())
		return false
	}
	s.log("✓ Contraseña enviada (recordada)")
	return true
}

func (s *Server) persistCredentials(user, pass string, remember bool) {
	if remember {
		if user == "" || pass == "" {
			return
		}
		if _, _, err := core.SaveCredentials(user, pass); err != nil {
			s.log("No se pudieron guardar credenciales: " + err.Error())
		}
		return
	}
	_ = core.DeleteCredentials()
	s.credMu.Lock()
	s.savedUser = ""
	s.savedPass = ""
	s.credMu.Unlock()
}

func (s *Server) invalidateSavedFor(stage string) {
	s.credMu.Lock()
	switch stage {
	case "username":
		s.savedUser = ""
		s.savedPass = ""
	case "password":
		s.savedPass = ""
	default:
		s.credMu.Unlock()
		return
	}
	s.credMu.Unlock()
	_ = core.DeleteCredentials()
}

func (s *Server) saveConfig(path string) error {
	if path == "" {
		return errors.New("ruta vacía")
	}
	s.cfg.VPNConfigPath = path
	if err := s.cfg.Save(); err != nil {
		return err
	}
	s.log("✓ Configuración guardada: " + path)
	return nil
}

// --- Utilidad ---------------------------------------------------------------

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
