package ui

import "sync"

// hubEvent es el payload que el server envía al cliente por SSE.
// Los campos son opcionales según type.
type hubEvent struct {
	Type       string `json:"type"`
	Message    string `json:"message,omitempty"`
	State      string `json:"state,omitempty"`
	LocalTunIP string `json:"localTunIP,omitempty"`
	RemoteIP   string `json:"remoteIP,omitempty"`
	BytesIn    uint64 `json:"bytesIn,omitempty"`
	BytesOut   uint64 `json:"bytesOut,omitempty"`
	RateIn     uint64 `json:"rateIn,omitempty"`
	RateOut    uint64 `json:"rateOut,omitempty"`
	Stage      string `json:"stage,omitempty"`
}

// eventHub hace fan-out de eventos a múltiples clientes SSE.
// Si un cliente no consume a tiempo, se descarta el evento para él; el hub
// nunca se bloquea.
type eventHub struct {
	mu      sync.Mutex
	clients map[chan hubEvent]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{clients: make(map[chan hubEvent]struct{})}
}

// subscribe registra un nuevo cliente y devuelve su canal.
func (h *eventHub) subscribe() chan hubEvent {
	ch := make(chan hubEvent, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// unsubscribe quita un cliente y cierra su canal.
func (h *eventHub) unsubscribe(ch chan hubEvent) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// broadcast envía un evento a todos los clientes registrados (no bloqueante).
func (h *eventHub) broadcast(e hubEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- e:
		default:
			// Consumidor lento: descartamos el evento para no bloquear al hub.
		}
	}
}
