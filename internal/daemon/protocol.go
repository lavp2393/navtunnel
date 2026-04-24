// Package daemon define el protocolo IPC y el servidor de daemon para
// navtunnel-cli. El cliente TUI y el daemon se hablan por TCP loopback
// autenticado con cookie, usando líneas JSON (una por mensaje).
package daemon

import "encoding/json"

// Envelope es el marco común de todos los mensajes: un type discriminador
// y un payload JSON opaco cuyo shape depende del type.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// --- Comandos: cliente → daemon --------------------------------------------

const (
	CmdSubscribe         = "subscribe"         // iniciar el streaming de eventos
	CmdConnect           = "connect"           // arrancar el Manager
	CmdDisconnect        = "disconnect"        // detener el Manager
	CmdSubmitCredential  = "submit-credential" // responder a ask-user/pass/otp
	CmdSaveConfig        = "save-config"       // persistir .ovpn path
	CmdShutdown          = "shutdown"          // terminar el daemon
	CmdPing              = "ping"              // health check
)

// SubmitCredentialPayload acompaña a CmdSubmitCredential.
type SubmitCredentialPayload struct {
	Stage    string `json:"stage"` // "user" | "pass" | "otp"
	Value    string `json:"value"`
	Remember bool   `json:"remember,omitempty"`
}

// SaveConfigPayload acompaña a CmdSaveConfig.
type SaveConfigPayload struct {
	OvpnPath string `json:"ovpnPath"`
}

// --- Eventos: daemon → cliente ----------------------------------------------

const (
	EvtHello       = "hello"       // primer mensaje tras auth; informa versión.
	EvtState       = "state"       // cambio de estado de OpenVPN.
	EvtBytecount   = "bytecount"   // contadores rx/tx + rates.
	EvtLog         = "log"         // línea de log.
	EvtAskUser     = "ask-user"    // pide usuario.
	EvtAskPass     = "ask-pass"    // pide password.
	EvtAskOTP      = "ask-otp"     // pide OTP.
	EvtConnected   = "connected"   // túnel ya activo.
	EvtDisconnect  = "disconnected"// túnel caído / cerrado.
	EvtAuthFailed  = "auth-failed" // credenciales rechazadas.
	EvtFatal       = "fatal"       // error no recuperable.
	EvtReply       = "reply"       // respuesta a un comando (ok/error).
)

// HelloPayload se manda tras autenticar el socket.
type HelloPayload struct {
	Version  string `json:"version"`
	OvpnPath string `json:"ovpnPath,omitempty"`
}

// StatePayload describe el estado de conexión y metrics snapshot.
type StatePayload struct {
	State          string `json:"state"`
	LocalTunIP     string `json:"localTunIP,omitempty"`
	RemoteServerIP string `json:"remoteServerIP,omitempty"`
	BytesIn        uint64 `json:"bytesIn"`
	BytesOut       uint64 `json:"bytesOut"`
	BytesInRate    uint64 `json:"bytesInRate"`
	BytesOutRate   uint64 `json:"bytesOutRate"`
}

// BytecountPayload es el update incremental que llega en cada bytecount.
type BytecountPayload struct {
	BytesIn  uint64 `json:"bytesIn"`
	BytesOut uint64 `json:"bytesOut"`
	RateIn   uint64 `json:"rateIn"`
	RateOut  uint64 `json:"rateOut"`
}

// LogPayload es una línea suelta.
type LogPayload struct {
	Line string `json:"line"`
}

// AskPayload para cualquiera de los ask-*.
type AskPayload struct {
	Message string `json:"message"`
}

// AuthFailedPayload incluye la etapa donde falló.
type AuthFailedPayload struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// FatalPayload mensaje de error crítico del Manager.
type FatalPayload struct {
	Message string `json:"message"`
}

// DisconnectedPayload razón del cierre.
type DisconnectedPayload struct {
	Reason string `json:"reason"`
}

// ReplyPayload es la respuesta a un comando del cliente.
type ReplyPayload struct {
	ID    string `json:"id,omitempty"`    // opcional, eco del CommandEnvelope.ID si se usa
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// CommandEnvelope extiende Envelope con un ID opcional para correlación.
type CommandEnvelope struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}
