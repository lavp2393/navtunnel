package linux

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestElevateCommandConsoleModeUsesSudoNonInteractive(t *testing.T) {
	binDir := t.TempDir()
	fakeSudo := filepath.Join(binDir, "sudo")
	if err := os.WriteFile(fakeSudo, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("creando sudo de prueba: %v", err)
	}
	t.Setenv("PATH", binDir)

	program, args, err := NewLinux().ElevateCommand("/usr/bin/openvpn", []string{"--config", "client.ovpn"}, true)
	if err != nil {
		t.Fatalf("ElevateCommand() error = %v", err)
	}
	if filepath.Base(program) != filepath.Base(fakeSudo) {
		t.Fatalf("program = %q, want executable %q", program, fakeSudo)
	}
	want := []string{"-n", "/usr/bin/openvpn", "--config", "client.ovpn"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}
