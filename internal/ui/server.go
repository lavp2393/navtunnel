package ui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// server expone la UI web vía HTTP local. La autenticación es un token
// compartido único por sesión que el tray injecta en la URL cuando abre el
// navegador.
type server struct {
	app    *App
	secret string
	http   *http.Server
	port   int
}

type stateResponse struct {
	OvpnPath   string   `json:"ovpnPath,omitempty"`
	Metrics    metrics  `json:"metrics"`
	RecentLogs []string `json:"recentLogs,omitempty"`
}

type metrics struct {
	State          string `json:"state,omitempty"`
	BytesIn        uint64 `json:"bytesIn"`
	BytesOut       uint64 `json:"bytesOut"`
	BytesInRate    uint64 `json:"bytesInRate"`
	BytesOutRate   uint64 `json:"bytesOutRate"`
	LocalTunnelIP  string `json:"localTunnelIP,omitempty"`
	RemoteServerIP string `json:"remoteServerIP,omitempty"`
}

type credentialRequest struct {
	Stage    string `json:"stage"`
	Value    string `json:"value"`
	Remember bool   `json:"remember"`
}

type configRequest struct {
	OvpnPath string `json:"ovpnPath"`
}

// newServer prepara el HTTP listener. No inicia aún; usar start().
func newServer(app *App) (*server, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := l.Addr().(*net.TCPAddr)

	secret, err := randomToken(16)
	if err != nil {
		_ = l.Close()
		return nil, err
	}

	s := &server{app: app, secret: secret, port: addr.Port}
	mux := http.NewServeMux()
	mux.Handle("/", s.assetsHandler())
	mux.HandleFunc("/api/state", s.auth(s.handleState))
	mux.HandleFunc("/api/events", s.auth(s.handleEvents))
	mux.HandleFunc("/api/connect", s.auth(s.handleConnect))
	mux.HandleFunc("/api/disconnect", s.auth(s.handleDisconnect))
	mux.HandleFunc("/api/credential", s.auth(s.handleCredential))
	mux.HandleFunc("/api/config", s.auth(s.handleConfig))
	mux.HandleFunc("/api/pick-file", s.auth(s.handlePickFile))

	s.http = &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE necesita escritura larga.
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		_ = s.http.Serve(l)
	}()
	return s, nil
}

// url devuelve la URL que hay que abrir en el navegador, con token injectado.
func (s *server) url() string {
	return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", s.port, s.secret)
}

// shutdown cierra el HTTP server con un timeout corto.
func (s *server) shutdown() {
	if s.http == nil {
		return
	}
	_ = s.http.Close()
}

// assetsHandler sirve los archivos embebidos. Los assets no llevan token:
// son estáticos y útiles solo con el HTML principal que sí lo exige.
func (s *server) assetsHandler() http.Handler {
	return http.FileServer(http.FS(webRoot()))
}

// auth envuelve un handler pidiendo el token (query o header) y comparándolo
// en tiempo constante contra el secreto de sesión.
func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("t")
		if got == "" {
			got = r.Header.Get("X-Auth-Token")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- Handlers ---------------------------------------------------------------

func (s *server) handleState(w http.ResponseWriter, _ *http.Request) {
	resp := stateResponse{
		OvpnPath:   s.app.currentOvpnPath(),
		Metrics:    s.app.currentMetrics(),
		RecentLogs: s.app.recentLogs(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.app.hub.subscribe()
	defer s.app.hub.unsubscribe(ch)

	// Heartbeat para detectar disconnect y mantener el canal vivo.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) handleConnect(w http.ResponseWriter, _ *http.Request) {
	if err := s.app.connect(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleDisconnect(w http.ResponseWriter, _ *http.Request) {
	s.app.disconnect()
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleCredential(w http.ResponseWriter, r *http.Request) {
	var req credentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.app.submitCredential(req.Stage, req.Value, req.Remember); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.app.saveOvpnPath(req.OvpnPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlePickFile(w http.ResponseWriter, _ *http.Request) {
	path, err := pickOvpnFile()
	if err != nil {
		if errors.Is(err, errNoPicker) {
			http.Error(w, "no native picker available", http.StatusNotImplemented)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

// --- helpers ----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
