package core

import (
	"bufio"
	"fmt"
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

	// 4. Iniciar el lector del PTY en una goroutine
	m.wg.Add(1)
	go m.readPTY()

	// Goroutine para manejar el fin del proceso
	go func() {
		m.cmd.Wait()
		// Si el proceso termina, avisamos
		select {
		case m.events <- Event{Type: EventDisconnected, Message: "Proceso OpenVPN terminado"}:
		case <-m.stopCh:
		}
		m.Stop() // Asegurarse de cerrar todo
	}()

	return m, nil
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

// Stop detiene el manager y mata el proceso OpenVPN
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.stopCh:
		// Ya está cerrado
		return
	default:
		close(m.stopCh)

		// Cerrar el PTY primero
		if m.ptmx != nil {
			m.ptmx.Close()
		}

		if m.cmd != nil && m.cmd.Process != nil {
			// Intentar terminarlo limpiamente
			m.cmd.Process.Kill()
		}
		m.wg.Wait()
		close(m.events)
	}
}

// sendCommand envía un comando (credencial) al PTY
func (m *Manager) sendCommand(cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ptmx == nil {
		return fmt.Errorf("PTY no está disponible")
	}
	// Escribimos la credencial seguida de un salto de línea
	_, err := m.ptmx.Write([]byte(cmd + "\n"))
	return err
}

// readPTY lee continuamente del pseudo-terminal
// IMPORTANTE: No usamos Scanner porque los prompts de OpenVPN no tienen newline
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
			// Leer con timeout implícito (bloqueante pero responsive)
			n, err := reader.Read(buf)
			if err != nil {
				if err.Error() != "EOF" {
					// Error de lectura, terminar
					return
				}
			}

			if n > 0 {
				chunk := string(buf[:n])
				buffer.WriteString(chunk)

				// Procesar líneas completas (con \n)
				data := buffer.String()
				lines := strings.Split(data, "\n")

				// La última parte puede ser incompleta (sin \n)
				buffer.Reset()
				if !strings.HasSuffix(data, "\n") {
					// Guardamos la línea incompleta en el buffer
					buffer.WriteString(lines[len(lines)-1])
					lines = lines[:len(lines)-1]
				}

				// Procesar líneas completas
				for _, line := range lines {
					if line != "" {
						m.events <- Event{
							Type:    EventLogLine,
							Message: line,
						}
						m.parseLine(line)
					}
				}

				// Procesar línea incompleta si contiene prompts conocidos
				incomplete := buffer.String()
				if incomplete != "" {
					// Detectar prompts sin newline
					if strings.Contains(incomplete, "Enter Auth Username:") ||
						strings.Contains(incomplete, "Enter Auth Password:") ||
						strings.Contains(incomplete, "CHALLENGE:") ||
						strings.HasSuffix(incomplete, "Response:") {
						m.events <- Event{
							Type:    EventLogLine,
							Message: incomplete,
						}
						m.parseLine(incomplete)
						buffer.Reset()
					}
				}
			}
		}
	}
}

// parseLine "raspa" la salida de la consola para encontrar prompts
func (m *Manager) parseLine(line string) {
	line = strings.TrimSpace(line)

	// --- Lógica de Detección de Prompts (Basada en tu captura) ---

	// 1. Pedir Usuario
	// Cambiado de HasSuffix a Contains porque OpenVPN imprime el input en la misma línea
	if strings.Contains(line, "Enter Auth Username:") {
		m.mu.Lock()
		m.currentStage = "username"
		m.mu.Unlock()
		m.events <- Event{
			Type:    EventAskUser,
			Message: "Ingresa tu usuario corporativo",
		}
		return
	}

	// 2. Pedir Contraseña
	// Cambiado de HasSuffix a Contains porque OpenVPN imprime el input en la misma línea
	if strings.Contains(line, "Enter Auth Password:") {
		m.mu.Lock()
		m.currentStage = "password"
		m.mu.Unlock()
		m.events <- Event{
			Type:    EventAskPass,
			Message: "Ingresa tu contraseña",
		}
		return
	}

	// 3. Pedir OTP - Múltiples variaciones posibles
	// Detectar diferentes formatos de challenge de OTP
	if strings.HasPrefix(line, "CHALLENGE:") ||
		strings.Contains(line, "static challenge") ||
		strings.Contains(line, "Static challenge") ||
		(strings.Contains(line, "OTP") && strings.Contains(line, "Enter")) ||
		strings.HasSuffix(line, "Response:") {
		m.mu.Lock()
		m.currentStage = "otp"
		m.mu.Unlock()

		// Extraer el mensaje del challenge si queremos
		msg := "Ingresa tu código OTP"
		if strings.HasPrefix(line, "CHALLENGE:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				msg = strings.TrimSpace(parts[1])
			}
		}

		m.events <- Event{
			Type:    EventAskOTP,
			Message: msg,
		}
		return
	}

	// --- Lógica de Detección de Estado ---

	// 4. Fallo de Autenticación
	if strings.Contains(line, "AUTH_FAILED") {
		m.mu.Lock()
		stage := m.currentStage
		m.mu.Unlock()

		m.events <- Event{
			Type:    EventAuthFailed,
			Message: getAuthFailedMessage(stage),
			Stage:   stage,
		}
		return
	}

	// 5. Conexión Exitosa
	// Este es el mensaje más común cuando la VPN se establece
	if strings.Contains(line, "Initialization Sequence Completed") {
		m.events <- Event{
			Type:    EventConnected,
			Message: "Conexión establecida ✅",
		}
		return
	}

	// 6. Error Fatal
	if strings.HasPrefix(line, "FATAL:") {
		m.events <- Event{
			Type:    EventFatal,
			Message: strings.TrimPrefix(line, "FATAL:"),
		}
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

// FindFreePort ya no es necesario para este método.
// Lo comento por si lo necesitas en otro lado, pero este manager no lo usa.
/*
func FindFreePort() (int, error) {
        rand.Seed(time.Now().UnixNano())
        for i := 0; i < 10; i++ {
                port := 49152 + rand.Intn(65535-49152)
                ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
                if err == nil {
                        ln.Close()
                        return port, nil
                }
        }
        return 0, fmt.Errorf("no se pudo encontrar un puerto libre")
}
*/
