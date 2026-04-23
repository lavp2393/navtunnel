package main

import (
	"os"

	"github.com/lavp2393/navtunnel/internal/ui"
)

func main() {
	// Modo subproceso: el proceso principal hace spawn de una copia de sí
	// mismo con "--panel <url>" para abrir la ventana nativa del webview.
	// Aislar el webview en su propio proceso evita conflictos de main-thread
	// con systray en macOS y libera toda la RAM del webview al cerrar la
	// ventana.
	if len(os.Args) > 1 && os.Args[1] == "--panel" {
		url := ""
		if len(os.Args) > 2 {
			url = os.Args[2]
		}
		ui.RunPanel(url)
		return
	}
	ui.NewApp().Run()
}
