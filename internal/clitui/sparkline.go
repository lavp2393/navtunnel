package clitui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bar es el set de caracteres para sparklines verticales de 8 niveles.
var bars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline toma la última `width` valores del buffer (pad con 0 por la
// izquierda si faltan) y los renderiza con los caracteres de bloque como
// un minigráfico. style tinta cada celda con el mismo foreground.
func Sparkline(buf []float64, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	vals := make([]float64, width)
	start := len(buf) - width
	if start < 0 {
		copy(vals[-start:], buf)
	} else {
		copy(vals, buf[start:])
	}

	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return style.Render(strings.Repeat("▁", width))
	}

	var sb strings.Builder
	for _, v := range vals {
		idx := int((v / max) * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		sb.WriteRune(bars[idx])
	}
	return style.Render(sb.String())
}
