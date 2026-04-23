package ui

import (
	"log"

	webview "github.com/webview/webview_go"
)

// RunPanel crea una ventana nativa (WKWebView en macOS, WebView2 en Windows,
// WebKit2GTK en Linux) apuntada a la URL indicada. Se ejecuta en el subproceso
// "--panel" del binario para que el webview viva y muera con la ventana sin
// afectar al proceso principal (tray + servidor HTTP).
//
// Este ciclo-de-vida aislado es importante: el webview consume RAM solo mientras
// la ventana está abierta, y tanto macOS como Windows requieren que el webview
// corra en el main thread del proceso; el subproceso dedicado evita conflictos
// con systray (que también se apropia del main thread).
func RunPanel(url string) {
	if url == "" {
		log.Fatal("navtunnel --panel: URL requerida")
	}
	w := webview.New(false)
	if w == nil {
		log.Fatal("no se pudo crear la ventana nativa (faltaría webkit2gtk en Linux?)")
	}
	defer w.Destroy()
	w.SetTitle("NavTunnel")
	w.SetSize(960, 740, webview.HintNone)
	w.Navigate(url)
	w.Run()
}
