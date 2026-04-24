package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

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
	if pingExistingDaemon() {
		return nil
	}
	// Borrar info viejo si existe (daemon murió sin limpiar).
	removeInfo()

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "NAVTUNNEL_CLI_DAEMON=1")
	// Redirigir stdout/stderr/stdin a /dev/null para que el daemon no
	// herede el tty del cliente ni retenga FDs bloqueantes.
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devnull
		cmd.Stdout = devnull
		cmd.Stderr = devnull
		defer devnull.Close()
	}
	detachFromParent(cmd)
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

