// navtunnel-cli es el cliente de terminal: muestra una TUI (bubbletea)
// para controlar la VPN. Al primer arranque forkea un subproceso
// "navtunnel-cli --daemon" que mantiene la conexión viva; la TUI se
// conecta por loopback TCP autenticado con cookie. Cerrar la TUI con "q"
// NO termina el daemon — la próxima invocación de navtunnel-cli reattachea
// al mismo daemon y muestra el estado actual.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lavp2393/navtunnel/internal/clitui"
	"github.com/lavp2393/navtunnel/internal/daemon"
)

var version = "dev"

func main() {
	// Modo daemon: se activa por env var en vez de flag, así el usuario
	// nunca tipea "--daemon" y el help no lo expone. El cliente seta la
	// variable al spawnear el subproceso (ver daemon.EnsureDaemon).
	if os.Getenv("NAVTUNNEL_CLI_DAEMON") == "1" {
		runDaemon()
		return
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("navtunnel-cli", version)
			return
		case "--quit":
			runQuit()
			return
		case "--help", "-h":
			printHelp()
			return
		default:
			fmt.Fprintln(os.Stderr, "flag desconocido:", os.Args[1])
			printHelp()
			os.Exit(2)
		}
	}
	runClient()
}

func printHelp() {
	fmt.Println(`navtunnel-cli — cliente OpenVPN con TUI

Uso:
  navtunnel-cli             Abrir la TUI (inicia el servicio en segundo plano
                            automáticamente si aún no está corriendo)
  navtunnel-cli --quit      Apagar el servicio (desconecta la VPN)
  navtunnel-cli --version   Mostrar versión
  navtunnel-cli --help      Esta ayuda

Al cerrar la TUI con "q", el servicio sigue corriendo en segundo plano
para mantener la conexión. Volvé a ejecutar navtunnel-cli para verla o
controlarla.`)
}

func runClient() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo localizar el ejecutable:", err)
		os.Exit(1)
	}
	if err := daemon.EnsureDaemon(exe); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo iniciar el daemon:", err)
		os.Exit(1)
	}
	client, err := daemon.Connect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo conectar al daemon:", err)
		os.Exit(1)
	}
	defer client.Close()

	if err := clitui.Run(client); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}

func runDaemon() {
	srv, err := daemon.NewServer(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon init:", err)
		os.Exit(1)
	}

	// Señales: SIGTERM/SIGINT apagan el daemon.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		srv.Shutdown()
	}()

	if err := srv.Serve(); err != nil {
		fmt.Fprintln(os.Stderr, "daemon serve:", err)
		os.Exit(1)
	}
}

func runQuit() {
	client, err := daemon.Connect()
	if err != nil {
		fmt.Println("no hay daemon corriendo")
		return
	}
	defer client.Close()
	if err := client.Send(daemon.CmdShutdown, nil); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown:", err)
		os.Exit(1)
	}
	fmt.Println("daemon apagado")
}
