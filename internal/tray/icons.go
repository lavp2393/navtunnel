package tray

import _ "embed"

// Iconos embebidos en el binario
// Los archivos PNG deben estar en internal/tray/icons/

//go:embed icons/disconnected.png
var IconDisconnectedData []byte

//go:embed icons/connecting.png
var IconConnectingData []byte

//go:embed icons/connected.png
var IconConnectedData []byte

//go:embed icons/error.png
var IconErrorData []byte

// GetIconData devuelve los bytes del icono según el tipo
func GetIconData(iconType IconType) []byte {
	switch iconType {
	case IconDisconnected:
		return IconDisconnectedData
	case IconConnecting:
		return IconConnectingData
	case IconConnected:
		return IconConnectedData
	case IconError:
		return IconErrorData
	default:
		return IconDisconnectedData
	}
}
