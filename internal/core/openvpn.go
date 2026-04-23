package core

import "github.com/lavp2393/navtunnel/internal/platform"

// FindOpenVPN localiza el binario openvpn según la plataforma activa.
func FindOpenVPN() (string, error) {
	return platform.New().FindOpenVPN()
}
