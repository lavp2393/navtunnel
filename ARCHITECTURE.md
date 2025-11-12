# PreyVPN - Arquitectura Multi-Plataforma

## Última actualización: 2025-11-11

---

## Visión General

PreyVPN es un cliente OpenVPN con interfaz gráfica diseñado para usuarios no técnicos, que soporta múltiples plataformas mediante una arquitectura modular y abstracciones específicas por sistema operativo.

**Filosofía de diseño:**
- 100% GUI, cero uso de terminal
- Configuración persistente y visual
- System tray nativo por plataforma
- Instalación automática de dependencias y permisos

### Plataformas Soportadas

| Plataforma | Estado | Arquitecturas |
|------------|--------|---------------|
| **Linux** | ✅ Completo | amd64, arm64 |
| **Windows** | 🚧 En desarrollo | amd64, arm64 |
| **macOS** | 🚧 En desarrollo | amd64 (Intel), arm64 (Apple Silicon) |

---

## Estructura del Proyecto

```
binariovpnprey/
├── cmd/
│   └── preyvpn/
│       └── main.go                    # Entry point común para todas las plataformas
│
├── internal/
│   ├── core/
│   │   ├── manager.go                 # Management Interface (común)
│   │   └── openvpn.go                 # Wrapper que usa platform abstraction
│   │
│   ├── platform/                      # ⭐ Abstracciones por plataforma
│   │   ├── platform.go                # Interface común
│   │   ├── platform_linux.go          # Build tags para Linux
│   │   ├── platform_windows.go        # Build tags para Windows
│   │   ├── platform_darwin.go         # Build tags para macOS
│   │   │
│   │   ├── linux/
│   │   │   └── linux.go               # Implementación completa para Linux
│   │   │
│   │   ├── windows/
│   │   │   └── windows.go             # Stub con TODOs
│   │   │
│   │   └── darwin/
│   │       └── darwin.go              # Stub con TODOs
│   │
│   ├── tray/                          # ⭐ System tray abstraction (NEW)
│   │   ├── tray.go                    # Interface común TrayIcon
│   │   ├── systray.go                 # Implementación con getlantern/systray
│   │   ├── appindicator.go            # [FUTURO] Implementación nativa AppIndicator3
│   │   └── icons/
│   │       ├── generate_icons.py      # Generador de iconos de estado
│   │       ├── disconnected.png       # Gris
│   │       ├── connecting.png         # Naranja
│   │       ├── connected.png          # Verde
│   │       └── error.png              # Rojo
│   │
│   ├── config/                        # ⭐ Configuración persistente (NEW)
│   │   └── config.go                  # Gestión de config.json (~/.config/PreyVPN/)
│   │
│   ├── ui/
│   │   ├── app.go                     # UI común (Fyne es cross-platform)
│   │   └── prompts.go                 # Modales de entrada + file picker
│   │
│   └── logs/
│       └── buffer.go                  # Buffer circular de logs
│
├── packaging/                         # ⭐ Packaging para distribución (NEW)
│   ├── build-deb.sh                   # Script de construcción .deb
│   ├── create-icon.py                 # Generador de icono para .deb
│   └── debian/
│       ├── DEBIAN/
│       │   ├── control                # Metadata y dependencias
│       │   ├── postinst               # Configura sudo NOPASSWD automáticamente
│       │   └── prerm                  # Limpieza en desinstalación
│       └── usr/
│           ├── bin/                   # Destino del binario
│           └── share/
│               ├── applications/
│               │   └── preyvpn.desktop
│               └── icons/hicolor/256x256/apps/
│                   └── preyvpn.png
│
├── build/                             # Scripts de build por plataforma
│   ├── linux/
│   ├── windows/
│   └── darwin/
│
├── dist/                              # Binarios compilados y paquetes
│   ├── preyvpn                        # Binario Linux
│   ├── preyvpn_1.0.0_amd64.deb       # Paquete Debian
│   ├── linux-amd64/
│   ├── linux-arm64/
│   ├── windows-amd64/
│   ├── windows-arm64/
│   ├── darwin-amd64/
│   └── darwin-arm64/
│
├── configs/                           # Configuraciones por plataforma
│   ├── linux/
│   │   └── preyvpn.desktop           # Desktop entry para Linux
│   ├── windows/
│   │   └── README.md                 # Guía para iconos, manifests, etc.
│   └── darwin/
│       └── Info.plist                # App bundle info para macOS
│
├── Makefile                           # Build system multi-plataforma
├── Taskfile.yml                       # Task runner para Docker builds
├── dev.sh                             # Script de desarrollo
├── go.mod
├── README.md
├── ARCHITECTURE.md                    # Este archivo
├── BUILD.md                           # Documentación de compilación
├── USAGE.md                           # Guía de uso
├── PreyVPN_Spec_MVP.md
└── TECHNICAL_CONTEXT.md
```

---

## Abstracción de Plataforma

### Interface `platform.Platform`

Define el contrato común que todas las plataformas deben implementar:

```go
type Platform interface {
    // Process management
    FindOpenVPN() (string, error)
    StartOpenVPN(config StartConfig) (*Process, error)
    StopOpenVPN(proc *Process) error

    // Privilege elevation
    RequiresElevation() bool
    ElevateCommand(path string, args []string) (string, []string, error)

    // Paths
    GetConfigDir() string
    GetDefaultConfigPath() string
    GetLogPath() string

    // Platform info
    Name() string
    Separator() string
}
```

### Selección Automática de Plataforma

El código usa **build tags** de Go para compilar solo la implementación correcta:

```go
// internal/platform/platform.go
func New() Platform {
    switch runtime.GOOS {
    case "linux":
        return NewLinux()
    case "windows":
        return NewWindows()
    case "darwin":
        return NewDarwin()
    }
}
```

Los archivos `platform_*.go` tienen build tags:
- `//go:build linux` → `platform_linux.go`
- `//go:build windows` → `platform_windows.go`
- `//go:build darwin` → `platform_darwin.go`

---

## System Tray Abstraction

PreyVPN incluye una abstracción para el system tray que permite cambiar de implementación sin afectar el resto del código.

### Interface `tray.TrayIcon`

```go
type TrayIcon interface {
    SetTitle(title string)
    SetIcon(iconType IconType)
    SetTooltip(text string)
    Run(onReady func(), onExit func())
    Quit()
}

type IconType int
const (
    IconDisconnected IconType = iota
    IconConnecting
    IconConnected
    IconError
)

type MenuCallbacks struct {
    OnConnect    func()
    OnDisconnect func()
    OnShow       func()
    OnQuit       func()
}
```

### Implementaciones Actuales y Futuras

#### Implementación Actual: getlantern/systray

**Archivo:** `internal/tray/systray.go`

Usa la librería [getlantern/systray](https://github.com/getlantern/systray) que funciona en:
- ✅ Linux (con AppIndicator como backend)
- ✅ Windows (system tray nativo)
- ✅ macOS (NSStatusBar)

**Ventajas:**
- Cross-platform desde el inicio
- No requiere CGo complejo
- Fácil de usar

**Limitaciones:**
- En Linux, integración limitada con GNOME Shell moderno
- Menos control sobre el menú y las notificaciones
- Puede tener problemas con algunos entornos de escritorio

#### Implementación Futura: AppIndicator3 Nativo (Recomendado para Linux)

**Archivo:** `internal/tray/appindicator.go` (pendiente)

Usará [libayatana-appindicator3](https://github.com/AyatanaIndicators/libayatana-appindicator) directamente con CGo.

**Ventajas:**
- ✅ Integración nativa con GNOME Shell
- ✅ Integración nativa con Ubuntu Unity
- ✅ Mejor soporte para menús contextuales
- ✅ Notificaciones nativas del sistema
- ✅ Mejor rendimiento en Linux

**Cómo implementar:**

1. **Crear el archivo** `internal/tray/appindicator.go`:

```go
package tray

// #cgo pkg-config: ayatana-appindicator3-0.1
// #include <libayatana-appindicator/app-indicator.h>
// #include <gtk/gtk.h>
import "C"

type AppIndicator struct {
    indicator *C.AppIndicator
    callbacks MenuCallbacks
    // ... campos del menú
}

func NewAppIndicator(callbacks MenuCallbacks) TrayIcon {
    // Implementación con CGo
}
```

2. **Modificar** `internal/tray/tray.go` para elegir implementación:

```go
// +build linux

func New(callbacks MenuCallbacks) TrayIcon {
    // Usar variable de entorno o flag para elegir
    if os.Getenv("PREYVPN_USE_APPINDICATOR") == "1" {
        return NewAppIndicator(callbacks)
    }
    return NewSystray(callbacks)
}
```

3. **Actualizar Dockerfile.build** con dependencias CGo:
```dockerfile
RUN apt-get install -y \
    libayatana-appindicator3-dev \
    libgtk-3-dev
```

#### System Tray en Windows (Futuro)

Para Windows, se puede usar el system tray nativo de Win32:

```go
// internal/tray/systray_windows.go
// +build windows

// Usar syscall para llamar a Shell_NotifyIcon
```

O continuar usando getlantern/systray que ya funciona bien en Windows.

#### System Tray en macOS (Futuro)

Para macOS, se puede usar NSStatusBar directamente:

```go
// internal/tray/systray_darwin.go
// +build darwin

// Usar Cocoa/Objective-C con CGo para NSStatusBar
```

O continuar usando getlantern/systray que ya funciona bien en macOS.

---

## Configuración Persistente

El sistema de configuración usa JSON para persistir preferencias del usuario.

**Ubicación por plataforma:**
- **Linux:** `~/.config/PreyVPN/config.json` (XDG Base Directory)
- **Windows:** `%APPDATA%\PreyVPN\config.json`
- **macOS:** `~/Library/Application Support/PreyVPN/config.json`

**Estructura actual:**

```go
type Config struct {
    VPNConfigPath string `json:"vpn_config_path"`
    Version       int    `json:"version"`
}
```

**Futuras extensiones:**
- Recordar usuario (con credenciales en keyring/Keychain)
- Múltiples perfiles VPN
- Preferencias de UI (idioma, tema, etc.)
- Opciones de auto-reconexión

---

## Packaging y Distribución

### Linux (.deb)

**Script:** `packaging/build-deb.sh`

El paquete .deb incluye:
- Binario en `/usr/bin/preyvpn`
- Desktop entry en `/usr/share/applications/`
- Icono en `/usr/share/icons/hicolor/256x256/apps/`
- Script `postinst` que configura `/etc/sudoers.d/preyvpn` automáticamente
- Script `prerm` que limpia la configuración

**Dependencias automáticas** (definidas en `debian/DEBIAN/control`):
```
openvpn, policykit-1, libgl1, libx11-6, libxrandr2, libxcursor1,
libxinerama1, libxi6, libxxf86vm1, libxrender1, libxfixes3, libxext6,
libxdamage1, libxcomposite1, libayatana-appindicator3-1, libdbus-1-3,
libglib2.0-0, libgtk-3-0, libcairo2, libpango-1.0-0
```

**Ventaja:** El usuario solo ejecuta `sudo dpkg -i preyvpn.deb` y todo se configura automáticamente.

### Windows (.msi) - Futuro

Usar WiX Toolset para crear un instalador MSI que:
- Instale el binario en `C:\Program Files\PreyVPN\`
- Cree entrada en el menú de inicio
- Configure permisos para OpenVPN
- Registre el servicio si es necesario

### macOS (.dmg + .app) - Futuro

Crear un bundle .app con:
- `PreyVPN.app/Contents/MacOS/preyvpn` (binario)
- `Info.plist` con metadata
- Iconos ICNS
- Firmar con certificado de desarrollador
- Crear .dmg para distribución

---

## Diferencias por Plataforma

### Linux (Completo)

| Aspecto | Implementación |
|---------|----------------|
| **OpenVPN Path** | `/usr/sbin/openvpn`, `/usr/bin/openvpn` |
| **Config Dir** | `~/.config/PreyVPN` (XDG spec) o `~/PreyVPN` (MVP) |
| **Log Path** | `~/.cache/PreyVPN/logs` |
| **Elevation** | `pkexec` (PolicyKit) |
| **Packaging** | .deb, .rpm, AppImage (futuro) |
| **Desktop Entry** | `configs/linux/preyvpn.desktop` |

**Dependencias:**
- `openvpn`
- `policykit-1` (pkexec)
- `libgl1-mesa-dev`, `xorg-dev` (para Fyne)

### Windows (En desarrollo)

| Aspecto | Implementación |
|---------|----------------|
| **OpenVPN Path** | `C:\Program Files\OpenVPN\bin\openvpn.exe` |
| **Config Dir** | `%APPDATA%\PreyVPN` |
| **Log Path** | `%LOCALAPPDATA%\PreyVPN\logs` |
| **Elevation** | UAC / `runas` / ShellExecute |
| **Packaging** | .msi, .exe installer (NSIS/WiX) |
| **Icon** | `configs/windows/preyvpn.ico` |

**TODOs:**
- [ ] Implementar elevación con UAC
- [ ] Manejar rutas de Windows correctamente
- [ ] Probar con OpenVPN GUI service
- [ ] Crear script de instalador NSIS

### macOS (En desarrollo)

| Aspecto | Implementación |
|---------|----------------|
| **OpenVPN Path** | `/usr/local/opt/openvpn/sbin/openvpn` (Homebrew) |
| **Config Dir** | `~/Library/Application Support/PreyVPN` |
| **Log Path** | `~/Library/Logs/PreyVPN` |
| **Elevation** | `osascript` (AppleScript) / SMJobBless |
| **Packaging** | .app bundle, .dmg |
| **Bundle Info** | `configs/darwin/Info.plist` |

**TODOs:**
- [ ] Implementar elevación con osascript
- [ ] Crear .app bundle correctamente
- [ ] Firmar código (para distribución)
- [ ] Probar en Apple Silicon (arm64)

---

## Build System

### Comandos Principales

```bash
# Desarrollo (plataforma actual)
make build          # Compilar para la plataforma actual
make run            # Compilar y ejecutar
make clean          # Limpiar archivos generados

# Multi-plataforma
make build-all      # Compilar para Linux, Windows, macOS (arch principal)
make build-all-arch # Compilar para todas las arquitecturas

# Específico por plataforma
make build-linux    # Linux amd64
make build-windows  # Windows amd64
make build-darwin   # macOS amd64 + arm64

# Utilidades
make info           # Mostrar información del sistema
make check-deps     # Verificar dependencias (Linux)
make help           # Ayuda completa
```

### Variables de Entorno

```bash
VERSION=v1.0.0 make build-release
```

---

## Cross-Compilation

Go soporta cross-compilation de forma nativa:

```bash
# Desde Linux, compilar para Windows
GOOS=windows GOARCH=amd64 go build -o preyvpn.exe cmd/preyvpn/main.go

# Desde Linux, compilar para macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o preyvpn cmd/preyvpn/main.go
```

### Limitaciones de Cross-Compilation

- **CGO**: Fyne requiere CGO, así que necesitas cross-compilers:
  - Linux → Windows: `mingw-w64`
  - Linux → macOS: `osxcross`
- **Pruebas**: Solo se puede probar en la plataforma nativa

---

## Flujo de Integración

### Añadir Soporte para Nueva Plataforma

1. **Crear implementación:** `internal/platform/<os>/<os>.go`
2. **Implementar interface:** Todos los métodos de `platform.Platform`
3. **Crear build tag:** `internal/platform/platform_<os>.go`
4. **Añadir target al Makefile:** `build-<os>`
5. **Configuración:** Añadir archivos en `configs/<os>/`
6. **Documentar:** Actualizar este archivo

### Probar en Múltiples Plataformas

```bash
# CI/CD debería probar en cada plataforma nativa
# Ejemplo con GitHub Actions:
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
```

---

## Roadmap

### ✅ v1.0 - Linux MVP (Completo)
- [x] Arquitectura multi-plataforma con abstracciones
- [x] Implementación completa para Linux
- [x] System tray con iconos de estado
- [x] File picker visual para selección de .ovpn
- [x] Configuración persistente (JSON)
- [x] Packaging .deb con configuración automática de sudo
- [x] Desktop entry y menú de aplicaciones
- [x] Multi-factor authentication (usuario + password + OTP)
- [x] Minimizar a tray en lugar de cerrar

### 🔄 Próximos Pasos

#### v1.1 - System Tray Nativo (Corto Plazo)
- [ ] Implementar `internal/tray/appindicator.go` con CGo
- [ ] Migrar de getlantern/systray a AppIndicator3 nativo
- [ ] Mejor integración con GNOME Shell
- [ ] Notificaciones nativas del sistema
- [ ] Variable de entorno/flag para elegir implementación

#### v1.2 - Windows Support (Mediano Plazo)
- [ ] Implementar `internal/platform/windows/` completo
- [ ] System tray nativo de Windows (Win32 API)
- [ ] Elevación con UAC
- [ ] File picker nativo de Windows
- [ ] Instalador .msi con WiX Toolset
- [ ] Firmar código para Windows Defender
- [ ] Packaging con dependencias (OpenVPN, TAP driver)

#### v1.3 - macOS Support (Mediano Plazo)
- [ ] Implementar `internal/platform/darwin/` completo
- [ ] System tray con NSStatusBar (Cocoa)
- [ ] Elevación con osascript/SMJobBless
- [ ] File picker nativo de macOS
- [ ] Bundle .app con Info.plist
- [ ] Crear .dmg para distribución
- [ ] Firmar y notarizar para Gatekeeper
- [ ] Soporte completo para Apple Silicon (arm64)

#### v2.0 - Features Avanzadas (Largo Plazo)
- [ ] Soporte de múltiples perfiles VPN
- [ ] Selector visual de perfiles
- [ ] Recordar usuario con keyring/Credential Manager/Keychain
- [ ] Auto-reconexión con backoff exponencial
- [ ] Reglas polkit por grupo (sin prompt)
- [ ] Auto-update system multiplataforma
- [ ] Logging configurable con niveles
- [ ] Estadísticas de uso (tiempo conectado, datos)
- [ ] Modo "headless" (sin GUI, solo tray)
- [ ] API REST local para integración con otros tools

---

## Referencias

### Documentación Técnica
- [OpenVPN Management Interface](https://openvpn.net/community-resources/management-interface/)
- [Go Build Tags](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
- [Fyne Cross-Platform](https://developer.fyne.io/started/)

### Herramientas de Packaging
- **Linux**: [fpm](https://github.com/jordansissel/fpm), AppImageKit
- **Windows**: [NSIS](https://nsis.sourceforge.io/), [WiX](https://wixtoolset.org/)
- **macOS**: [create-dmg](https://github.com/create-dmg/create-dmg)

---

**Última revisión:** 2025-11-04
**Mantenedor:** Equipo PreyVPN
