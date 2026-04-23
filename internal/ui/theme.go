package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// cyberpunkTheme implementa fyne.Theme con paleta neón sobre fondo oscuro.
// Se aplica al Fyne app con Settings().SetTheme() al iniciar.
type cyberpunkTheme struct{}

var _ fyne.Theme = (*cyberpunkTheme)(nil)

func (t *cyberpunkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{0x06, 0x06, 0x11, 0xFF}
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameMenuBackground:
		return color.NRGBA{0x10, 0x12, 0x2A, 0xFF}
	case theme.ColorNameDisabled:
		return color.NRGBA{0x3A, 0x42, 0x60, 0xFF}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{0x14, 0x18, 0x32, 0xFF}
	case theme.ColorNameForeground:
		return color.NRGBA{0xD8, 0xE6, 0xFF, 0xFF}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{0x6C, 0x7F, 0xA8, 0xFF}
	case theme.ColorNamePrimary:
		return color.NRGBA{0x00, 0xF0, 0xFF, 0xFF} // cian
	case theme.ColorNameHover:
		return color.NRGBA{0x00, 0xF0, 0xFF, 0x20}
	case theme.ColorNameFocus:
		return color.NRGBA{0x00, 0xF0, 0xFF, 0x40}
	case theme.ColorNameSelection:
		return color.NRGBA{0x00, 0xF0, 0xFF, 0x30}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{0x78, 0xDC, 0xFF, 0x2E}
	case theme.ColorNameError:
		return color.NRGBA{0xFF, 0x4D, 0x6D, 0xFF}
	case theme.ColorNameSuccess:
		return color.NRGBA{0x00, 0xFF, 0xA3, 0xFF}
	case theme.ColorNameWarning:
		return color.NRGBA{0xFF, 0xB8, 0x4D, 0xFF}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *cyberpunkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *cyberpunkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *cyberpunkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameText:
		return 13
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}

// Colores neón expuestos para widgets custom (chart, etc.).
var (
	colorCyan    = color.NRGBA{0x00, 0xF0, 0xFF, 0xFF}
	colorCyanDim = color.NRGBA{0x00, 0xF0, 0xFF, 0x50}
	colorMagenta = color.NRGBA{0xFF, 0x3D, 0xA0, 0xFF}
	colorMagDim  = color.NRGBA{0xFF, 0x3D, 0xA0, 0x50}
	colorPanel   = color.NRGBA{0x10, 0x12, 0x2A, 0xFF}
	colorGrid    = color.NRGBA{0x00, 0xF0, 0xFF, 0x14}
	colorDim     = color.NRGBA{0x6C, 0x7F, 0xA8, 0xFF}
)
