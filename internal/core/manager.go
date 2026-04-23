package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// EventType representa el tipo de evento
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
)

// Event representa un evento del proceso OpenVPN
type Event struct {
	Type    EventType
	Message string
	Stage   string // Para AuthFailed: "password" o "otp"
}

// SendFns agrupa las funciones para enviar credenciales
type SendFns struct {
	Username func(string) error
	Password func(string) error
	OTP      func(string) error
}

// Manager gestiona la comunicación con el proceso OpenVPN
type Manager struct {
	cmd          *exec.Cmd
	ptmx         *os.File // Pseudo-terminal master
	events       chan Event
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	currentStage string // "username", "password", "otp"
	mu           sync.Mutex
}

// Start inicia el manager y el proceso OpenVPN
// IMPORTANTE: ovpnPath es la ruta a tu archivo .ovpn
func Start(ovpnPath string, openvpnBinary string) (*Manager, error) {
	if openvpnBinary == "" {
		openvpnBinary = "openvpn"
	}

	// 1. Preparar el comando OpenVPN con elevación de privilegios
	// En Linux, OpenVPN necesita ejecutarse como root para crear el túnel
	// Usamos sudo porque pkexec bloquea stdin/stdout para interacción
	args := []string{
		"--config", ovpnPath,
		"--auth-nocache",
		"--auth-retry", "interact",
		"--verb", "3", // Verbosidad moderada
	}

	// Elevar con sudo
	// Asume que el usuario tiene NOPASSWD configurado para openvpn
	sudoArgs := append([]string{openvpnBinary}, args...)
	cmd := exec.Command("sudo", sudoArgs...)

	// 2. Crear un pseudo-terminal (PTY)
	// Esto simula un terminal interactivo real, evitando que OpenVPN
	// use systemd-ask-password
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("error al iniciar OpenVPN con PTY: %w", err)
	}

	// 3. Crear el Manager
	m := &Manager{
		cmd:    cmd,
		ptmx:   ptmx,
		events: make(chan Event, 100),
		stopCh: make(chan struct{}),
	}

	// 4. Iniciar las goroutines de I/O
	m.wg.Add(2)
	go m.readPTY()
	go m.waitProcess()

	return m, nil
}

// waitProcess espera a que el proceso termine y emite el evento correspondiente.
// Se contabiliza en m.wg para que Stop() pueda esperarla sin carreras al cerrar events.
func (m *Manager) waitProcess() {
	defer m.wg.Done()

	err := m.cmd.Wait()
	msg := "Proceso OpenVPN terminado"
	if err != nil && !isExpectedExitError(err) {
		msg = fmt.Sprintf("OpenVPN terminó: %v", err)
	}
	m.emit(Event{Type: EventDisconnected, Message: msg})
}

// isExpectedExitError devuelve true si el error de Wait() corresponde a una
// terminación esperada (p.ej. Kill() llamado por Stop()).
func isExpectedExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// emit envía un evento respetando stopCh y ante cierres concurrentes del canal.
// No bloquea indefinidamente si nadie consume: si el buffer está lleno y
// stopCh se dispara, descarta el evento.
func (m *Manager) emit(e Event) {
	defer func() {
		// Si alguien cerrara m.events (no debería pasar con el diseño actual),
		// recover evita tumbar el proceso.
		_ = recover()
	}()
	select {
	case m.events <- e:
	case <-m.stopCh:
	}
}

// Events retorna el canal de eventos
func (m *Manager) Events() <-chan Event {
	return m.events
}

// SendFunctions retorna las funciones para enviar credenciales
func (m *Manager) SendFunctions() SendFns {
	return SendFns{
		Username: func(username string) error {
			m.mu.Lock()
			m.currentStage = "password" // La siguiente etapa es password
			m.mu.Unlock()
			return m.sendCommand(username)
		},
		Password: func(password string) error {
			m.mu.Lock()
			m.currentStage = "otp" // La siguiente etapa es OTP
			m.mu.Unlock()
			return m.sendCommand(password)
		},
		OTP: func(otp string) error {
			m.mu.Lock()
			m.currentStage = "connected" // Ya no esperamos más credenciales
			m.mu.Unlock()
			return m.sendCommand(otp)
		},
	}
}

// Stop detiene el manager y mata el proceso OpenVPN.
// Es idempotente y seguro de llamar desde múltiples goroutines.
// IMPORTANTE: no toma m.mu mientras espera a las goroutines para evitar
// deadlock con readPTY/sendCommand que también usan m.mu.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)

		// Cerrar el PTY desbloquea la lectura en readPTY.
		if m.ptmx != nil {
			_ = m.ptmx.Close()
		}

		// Matar el proceso desbloquea cmd.Wait() en waitProcess.
		if m.cmd != nil && m.cmd.Process != nil {
			_ = m.cmd.Process.Kill()
		}

		m.wg.Wait()

		// Con readPTY y waitProcess terminadas, ya nadie emite eventos.
		// Seguro cerrar el canal para que los consumidores detecten el fin.
		close(m.events)
	})
}

// sendCommand envía un comando (credencial) al PTY.
// Rechaza valores que contengan caracteres de control de línea para evitar
// que un usuario/contraseña/OTP con \n o \r inyecte comandos extra al PTY
// y desincronice la máquina de estados.
func (m *Manager) sendCommand(cmd string) error {
	if strings.ContainsAny(cmd, "\n\r\x00") {
		return fmt.Errorf("credencial contiene caracteres de control no permitidos")
	}

	select {
	case <-m.stopCh:
		return fmt.Errorf("manager detenido")
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ptmx == nil {
		return fmt.Errorf("PTY no está disponible")
	}
	_, err := m.ptmx.Write([]byte(cmd + "\n"))
	return err
}

// readPTY lee continuamente del pseudo-terminal.
// IMPORTANTE: No usamos Scanner porque los prompts de OpenVPN no tienen newline.
func (m *Manager) readPTY() {
	defer m.wg.Done()

	reader := bufio.NewReader(m.ptmx)
	var buffer strings.Builder
	buf := make([]byte, 1024)

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			buffer.Write(buf[:n])

			data := buffer.String()
			lines := strings.Split(data, "\n")

			buffer.Reset()
			if !strings.HasSuffix(data, "\n") {
				// La última parte es incompleta; la guardamos para el próximo ciclo.
				buffer.WriteString(lines[len(lines)-1])
				lines = lines[:len(lines)-1]
			}

			for _, line := range lines {
				if line == "" {
					continue
				}
				m.emit(Event{Type: EventLogLine, Message: line})
				m.parseLine(line)
			}

			// Prompts sin newline (OpenVPN los escribe sin saltar línea).
			incomplete := buffer.String()
			if incomplete != "" && isKnownPrompt(incomplete) {
				m.emit(Event{Type: EventLogLine, Message: incomplete})
				m.parseLine(incomplete)
				buffer.Reset()
			}
		}

		if err != nil {
			// io.EOF o PTY cerrado: el proceso terminó o Stop() cerró el PTY.
			// En cualquier caso, salir; waitProcess emitirá EventDisconnected.
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return
			}
			// Cualquier otro error también es terminal para esta goroutine.
			return
		}
	}
}

// isKnownPrompt detecta fragmentos sin newline que ya son accionables.
func isKnownPrompt(s string) bool {
	return strings.Contains(s, "Enter Auth Username:") ||
		strings.Contains(s, "Enter Auth Password:") ||
		strings.Contains(s, "CHALLENGE:") ||
		strings.HasSuffix(s, "Response:")
}

// parseLine "raspa" la salida de la consola para encontrar prompts
func (m *Manager) parseLine(line string) {
	line = strings.TrimSpace(line)

	// --- Lógica de Detección de Prompts (Basada en tu captura) ---

	// 1. Pedir Usuario
	if strings.Contains(line, "Enter Auth Username:") {
		m.mu.Lock()
		m.currentStage = "username"
		m.mu.Unlock()
		m.emit(Event{Type: EventAskUser, Message: "Ingresa tu usuario corporativo"})
		return
	}

	// 2. Pedir Contraseña
	if strings.Contains(line, "Enter Auth Password:") {
		m.mu.Lock()
		m.currentStage = "password"
		m.mu.Unlock()
		m.emit(Event{Type: EventAskPass, Message: "Ingresa tu contraseña"})
		return
	}

	// 3. Pedir OTP — variaciones conocidas de OpenVPN.
	// Se usan prefijos/sufijos específicos para evitar falsos positivos
	// como "Cannot enter OTP mode".
	if strings.HasPrefix(line, "CHALLENGE:") ||
		strings.Contains(line, "static challenge") ||
		strings.Contains(line, "Static challenge") ||
		strings.Contains(line, "Enter OTP:") ||
		strings.HasSuffix(line, "Response:") {
		m.mu.Lock()
		m.currentStage = "otp"
		m.mu.Unlock()

		msg := "Ingresa tu código OTP"
		if strings.HasPrefix(line, "CHALLENGE:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				msg = strings.TrimSpace(parts[1])
			}
		}

		m.emit(Event{Type: EventAskOTP, Message: msg})
		return
	}

	// --- Lógica de Detección de Estado ---

	// 4. Fallo de Autenticación
	if strings.Contains(line, "AUTH_FAILED") {
		m.mu.Lock()
		stage := m.currentStage
		m.mu.Unlock()

		m.emit(Event{
			Type:    EventAuthFailed,
			Message: getAuthFailedMessage(stage),
			Stage:   stage,
		})
		return
	}

	// 5. Conexión Exitosa
	if strings.Contains(line, "Initialization Sequence Completed") {
		m.emit(Event{Type: EventConnected, Message: "Conexión establecida"})
		return
	}

	// 6. Error Fatal
	if strings.HasPrefix(line, "FATAL:") {
		m.emit(Event{Type: EventFatal, Message: strings.TrimPrefix(line, "FATAL:")})
		return
	}
}

// getAuthFailedMessage retorna el mensaje apropiado según la etapa
func getAuthFailedMessage(stage string) string {
	switch stage {
	case "password":
		return "Contraseña incorrecta"
	case "otp":
		return "OTP inválido o expirado"
	case "username":
		return "Usuario incorrecto"
	default:
		// Si el fallo ocurre después de enviar el OTP
		if stage == "connected" {
			return "OTP inválido o expirado"
		}
		return "Error de autenticación"
	}
}

