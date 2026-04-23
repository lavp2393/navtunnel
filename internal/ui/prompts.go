package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// credentialResult es lo que devuelve un prompt de usuario/contraseña.
type credentialResult struct {
	value    string
	remember bool
}

// promptCredential abre un diálogo modal con un input y opcional checkbox
// "Recordar credenciales". onConfirm recibe el resultado; si el usuario
// cancela, no se invoca.
func promptCredential(parent fyne.Window, title, message string, masked, showRemember bool, initialRemember bool, onConfirm func(credentialResult)) {
	var entry *widget.Entry
	if masked {
		entry = widget.NewPasswordEntry()
	} else {
		entry = widget.NewEntry()
	}
	entry.PlaceHolder = "..."

	remember := widget.NewCheck("Recordar credenciales", nil)
	remember.SetChecked(initialRemember)

	items := []fyne.CanvasObject{
		widget.NewLabel(message),
		entry,
	}
	if showRemember {
		items = append(items, remember)
	}
	content := container.NewVBox(items...)

	d := dialog.NewCustomConfirm(title, "Enviar", "Cancelar", content, func(ok bool) {
		if !ok {
			return
		}
		onConfirm(credentialResult{value: entry.Text, remember: remember.Checked})
	}, parent)
	d.Resize(fyne.NewSize(420, 200))
	d.Show()
	parent.Canvas().Focus(entry)
}
