package core

import (
	"sync"
	"time"
)

// Metrics contiene el estado y contadores de la conexión OpenVPN.
// Se actualiza desde el management interface vía >STATE: y >BYTECOUNT:.
type Metrics struct {
	// Estado actual reportado por openvpn (CONNECTING, AUTH, GET_CONFIG, ASSIGN_IP,
	// ADD_ROUTES, CONNECTED, RECONNECTING, EXITING, RESOLVE, TCP_CONNECT, WAIT).
	State string

	// Bytes acumulados desde el arranque de la conexión.
	BytesIn  uint64
	BytesOut uint64

	// Bytes por segundo (derivada entre dos muestras de BYTECOUNT).
	BytesInRate  uint64
	BytesOutRate uint64

	// IPs conocidas (pueden estar vacías antes de ASSIGN_IP/CONNECTED).
	LocalTunnelIP  string
	RemoteServerIP string

	// Momento en que transicionamos a CONNECTED por última vez.
	ConnectedSince time.Time
}

// metricsStore guarda el estado mutable bajo un RWMutex. No se expone
// directamente; el Manager sirve Snapshot() para lecturas seguras.
type metricsStore struct {
	mu        sync.RWMutex
	data      Metrics
	lastBytes struct {
		in, out uint64
		at      time.Time
	}
}

func newMetricsStore() *metricsStore {
	return &metricsStore{}
}

// Snapshot devuelve una copia de las métricas para uso seguro fuera del Manager.
func (s *metricsStore) Snapshot() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// applyState actualiza el estado y los IPs cuando llega un >STATE: del management.
func (s *metricsStore) applyState(state, localTun, remoteServer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.State
	s.data.State = state
	if localTun != "" {
		s.data.LocalTunnelIP = localTun
	}
	if remoteServer != "" {
		s.data.RemoteServerIP = remoteServer
	}
	if state == "CONNECTED" && prev != "CONNECTED" {
		s.data.ConnectedSince = time.Now()
	}
}

// applyBytecount actualiza los contadores y calcula la tasa instantánea.
// bytecount viene del management como "in,out" acumulados en bytes.
func (s *metricsStore) applyBytecount(in, out uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !s.lastBytes.at.IsZero() {
		dt := now.Sub(s.lastBytes.at).Seconds()
		if dt > 0 {
			if in >= s.lastBytes.in {
				s.data.BytesInRate = uint64(float64(in-s.lastBytes.in) / dt)
			}
			if out >= s.lastBytes.out {
				s.data.BytesOutRate = uint64(float64(out-s.lastBytes.out) / dt)
			}
		}
	}

	s.data.BytesIn = in
	s.data.BytesOut = out
	s.lastBytes.in = in
	s.lastBytes.out = out
	s.lastBytes.at = now
}
