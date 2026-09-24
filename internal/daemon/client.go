package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/term"
)

var errDaemonStatusUnsupported = errors.New("daemon status command unsupported")

// Client representa la conexión del TUI al daemon.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder

	// Events es el canal donde el caller recibe todos los eventos del daemon
	// (hello, state, bytecount, log, ask-*, connected, disconnected, etc.).
	// Se cierra cuando la conexión termina.
	Events chan Envelope
}

// Connect abre una conexión al daemon usando port + token del InfoFile.
func Connect() (*Client, error) {
	info, _, err := ReadInfo()
	if err != nil {
		return nil, err
	}
	return ConnectWith(info)
}

// ConnectWith es útil para tests / reusar un InfoFile leído previamente.
func ConnectWith(info InfoFile) (*Client, error) {
	if info.Port == 0 || info.Token == "" {
		return nil, errors.New("daemon info incompleto")
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(info.Port), 2*time.Second)
	if err != nil {
		return nil, err
	}
	// Handshake: mandar token.
	if _, err := conn.Write([]byte(info.Token + "\n")); err != nil {
		conn.Close()
		return nil, err
	}

	c := &Client{
		conn:   conn,
		enc:    json.NewEncoder(conn),
		dec:    json.NewDecoder(bufio.NewReader(conn)),
		Events: make(chan Envelope, 64),
	}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	defer close(c.Events)
	for {
		var env Envelope
		if err := c.dec.Decode(&env); err != nil {
			return
		}
		// Si el primer mensaje es un reply con auth=false, salimos.
		c.Events <- env
	}
}

// Send manda un comando al daemon.
func (c *Client) Send(cmd string, payload any) error {
	env := CommandEnvelope{Type: cmd}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		env.Payload = data
	}
	return c.enc.Encode(&env)
}

// Close cierra la conexión; el readLoop termina y Events se cierra.
func (c *Client) Close() error { return c.conn.Close() }

// --- Spawn del daemon -------------------------------------------------------

// EnsureDaemon garantiza que haya un daemon corriendo y escuchando. Si el
// InfoFile existe y el puerto responde ping, no hace nada. Si no, lanza
// un subproceso con NAVTUNNEL_CLI_DAEMON=1 desacoplado del cliente y
// espera hasta que aparezca el archivo + el socket responda, con timeout.
func EnsureDaemon(exe string) error {
	ttyPath, consoleTTY := currentConsoleTTY()
	if pingExistingDaemon() {
		info, _, err := ReadInfo()
		if err != nil || !consoleTTY || info.ConsoleTTY == ttyPath {
			return nil
		}

		connected, err := queryDaemonConnected(info)
		if err != nil {
			if errors.Is(err, errDaemonStatusUnsupported) {
				// Legacy daemons cannot report their Manager state. Keep them if
				// their OpenVPN process is present; otherwise rotate the idle daemon
				// so sudo can use the current terminal's authorization ticket.
				if hasManagedOpenVPN() {
					return nil
				}
				if _, err := sendDaemonRequest(info, CmdShutdown, "shutdown-legacy"); err != nil {
					return fmt.Errorf("no se pudo cerrar el daemon anterior: %w", err)
				}
				if err := waitDaemonExit(info.PID); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("el daemon pertenece a otra sesión SSH y no pude comprobar si la VPN sigue activa: %w; no lo detuve", err)
			}
		} else {
			if connected {
				// No interrumpir una VPN activa. Al salir y volver a abrir navt
				// cuando esté desconectada, se podrá asociar al TTY nuevo.
				return nil
			}
			stopped, err := shutdownDaemonIfIdle(info)
			if err != nil {
				return fmt.Errorf("no se pudo reiniciar el daemon de la sesión SSH anterior: %w", err)
			}
			if !stopped {
				return errors.New("el daemon inició una conexión mientras cambiaba la sesión SSH; vuelve a ejecutar navt")
			}
			if err := waitDaemonExit(info.PID); err != nil {
				return err
			}
		}
	}
	// Borrar info viejo si existe (daemon murió sin limpiar).
	removeInfo()

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "NAVTUNNEL_CLI_DAEMON=1")
	if consoleTTY {
		cmd.Env = append(cmd.Env, "NAVTUNNEL_CLI_TTY="+ttyPath)
	}
	// Redirigir stdout/stderr/stdin a /dev/null para que el daemon no
	// herede el tty del cliente ni retenga FDs bloqueantes.
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devnull
		cmd.Stdout = devnull
		cmd.Stderr = devnull
		defer devnull.Close()
	}
	detachFromParent(cmd, consoleTTY)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn daemon: %w", err)
	}
	pid := cmd.Process.Pid

	// Esperar a que publique el socket. Si el proceso hijo muere antes
	// del deadline, salir temprano con un error útil.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if pingExistingDaemon() {
			// Con el daemon vivo y respondiendo, liberar el Process para
			// que el kernel no guarde un zombie esperando wait().
			_ = cmd.Process.Release()
			return nil
		}
		// Si el proceso terminó sin haber respondido, no seguir esperando.
		if !processAlive(pid) {
			return fmt.Errorf("el proceso daemon (pid %d) salió antes de responder", pid)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("el daemon no arrancó a tiempo")
}

func currentConsoleTTY() (string, bool) {
	if runtime.GOOS != "linux" || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return "", false
	}
	path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", os.Stdin.Fd()))
	if err != nil || len(path) < len("/dev/") || path[:len("/dev/")] != "/dev/" {
		return "", false
	}
	return path, true
}

func queryDaemonConnected(info InfoFile) (bool, error) {
	reply, err := sendDaemonRequest(info, CmdStatus, "status")
	if err != nil {
		return false, err
	}
	if !reply.OK {
		if reply.Error == "unknown command: status" {
			return false, errDaemonStatusUnsupported
		}
		return false, errors.New(reply.Error)
	}
	return reply.Connected, nil
}

func shutdownDaemonIfIdle(info InfoFile) (bool, error) {
	reply, err := sendDaemonRequest(info, CmdShutdownIfIdle, "shutdown-if-idle")
	if err != nil {
		return false, err
	}
	if !reply.OK && reply.Connected {
		return false, nil
	}
	if !reply.OK {
		return false, errors.New(reply.Error)
	}
	return true, nil
}

func sendDaemonRequest(info InfoFile, command, id string) (ReplyPayload, error) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(info.Port), 2*time.Second)
	if err != nil {
		return ReplyPayload{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(info.Token + "\n")); err != nil {
		return ReplyPayload{}, err
	}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(CommandEnvelope{Type: command, ID: id}); err != nil {
		return ReplyPayload{}, err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	for {
		var env Envelope
		if err := dec.Decode(&env); err != nil {
			return ReplyPayload{}, err
		}
		if env.Type != EvtReply {
			continue
		}
		var reply ReplyPayload
		if err := json.Unmarshal(env.Payload, &reply); err != nil {
			return ReplyPayload{}, err
		}
		if reply.ID == id {
			return reply, nil
		}
	}
}

func waitDaemonExit(pid int) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			removeInfo()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("el daemon anterior no terminó a tiempo; no inicié otro")
}

// pingExistingDaemon devuelve true si hay un daemon vivo reachable con el
// token publicado en el InfoFile.
func pingExistingDaemon() bool {
	info, _, err := ReadInfo()
	if err != nil || info.Port == 0 || info.Token == "" {
		return false
	}
	// Doble chequeo: si el PID del info no está vivo, el daemon murió.
	if info.PID > 0 && !processAlive(info.PID) {
		return false
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(info.Port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := conn.Write([]byte(info.Token + "\n")); err != nil {
		return false
	}
	br := bufio.NewReader(conn)
	if _, err := br.ReadString('\n'); err != nil {
		return false
	}
	return true
}
