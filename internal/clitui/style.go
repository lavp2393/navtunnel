package clitui

import "github.com/charmbracelet/lipgloss"

// Paleta cyberpunk: neón sobre negro profundo.
var (
	colorBg        = lipgloss.Color("#050510")
	colorPanel     = lipgloss.Color("#0f1130")
	colorBorder    = lipgloss.Color("#1f3a5c")
	colorCyan      = lipgloss.Color("#00f0ff")
	colorCyanDim   = lipgloss.Color("#0088aa")
	colorMagenta   = lipgloss.Color("#ff3da0")
	colorMagDim    = lipgloss.Color("#aa2871")
	colorText      = lipgloss.Color("#d8e6ff")
	colorDim       = lipgloss.Color("#6c7fa8")
	colorOK        = lipgloss.Color("#00ffa3")
	colorWarn      = lipgloss.Color("#ffb84d")
	colorErr       = lipgloss.Color("#ff4d6d")
)

var (
	// Borde doble con esquinas "corner bracket" tipo HUD.
	borderStyle = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	panelStyle = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(colorBorder).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	dimTitleStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	cyanStyle    = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	magentaStyle = lipgloss.NewStyle().Foreground(colorMagenta).Bold(true)
	okStyle      = lipgloss.NewStyle().Foreground(colorOK).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	errStyle     = lipgloss.NewStyle().Foreground(colorErr).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)

	headerStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Padding(0, 1)

	brandAccent = lipgloss.NewStyle().Foreground(colorMagenta).Bold(true)

	tabActive = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true).
			Border(lipgloss.Border{Bottom: "━"}, false, false, true, false).
			BorderForeground(colorCyan).
			Padding(0, 2)

	tabInactive = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	logLineStyle = lipgloss.NewStyle().Foreground(colorDim)

	promptBox = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(colorCyan).
			Padding(1, 2).
			Foreground(colorText).
			Width(56)

	promptTitle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)
)

// dotForState devuelve el símbolo coloreado del estado principal.
func dotForState(state string) string {
	switch state {
	case "CONNECTED":
		return okStyle.Render("●")
	case "EXITING", "DISCONNECTED", "":
		return dimStyle.Render("○")
	case "ERROR":
		return errStyle.Render("●")
	default:
		return warnStyle.Render("◐")
	}
}
