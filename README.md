# NavTunnel - Cliente OpenVPN con GUI Multi-Plataforma

**Version 1.0.0 - Stable Release (Linux/Ubuntu - 2025-11-11)**

Cliente OpenVPN con interfaz gráfica diseñado para usuarios no técnicos, que facilita la conexión a la VPN corporativa mediante autenticación multi-factor (usuario + contraseña + OTP).

## Estado del Proyecto

| Plataforma | Estado | Arquitecturas |
|------------|--------|---------------|
| **Linux** | ✅ MVP Completo | amd64, arm64 |
| **Windows** | 🚧 En desarrollo | amd64, arm64 |
| **macOS** | 🚧 En desarrollo | amd64, arm64 |

> **Nota:** La implementación actual está enfocada en Linux/Ubuntu. El código está estructurado para soportar múltiples plataformas mediante abstracciones, con stubs preparados para Windows y macOS.

## Características

- **100% sin terminal**: Diseñado para usuarios no técnicos
- **File picker visual**: Selección gráfica del archivo .ovpn
- **System tray integration**: Icono persistente en la barra del sistema
  - Estados visuales: Desconectado (gris), Conectando (naranja), Conectado (verde), Error (rojo)
  - Menú contextual con Connect/Disconnect/Show/Quit
  - Minimizar a tray en lugar de cerrar
- **Configuración persistente**: Recuerda el archivo .ovpn seleccionado
- **Instalación con .deb**: Configura permisos automáticamente (no más sudo)
- **Autenticación multi-factor**: Usuario → Contraseña → OTP (LinOTP)
- **Gestión automática**: Maneja toda la comunicación con OpenVPN
- **Logs en vivo**: Visualización de eventos de conexión
- **Seguro**: No almacena credenciales (--auth-nocache)

## Requisitos del Sistema

### Sistema Operativo
- Ubuntu Desktop 20.04 o superior
- Otras distribuciones basadas en Debian (pueden funcionar)

### Dependencias (si usas el .deb, se instalan automáticamente)

- **OpenVPN**: Cliente VPN
- **PolicyKit**: Elevación de privilegios
- **Librerías GUI**: GTK3, Cairo, sistema de notificaciones
- **System Tray**: libayatana-appindicator3

Si instalas desde el paquete .deb, **todas las dependencias se instalan automáticamente**.

## Instalación

### Opción 1: Instalación con paquete .deb (Recomendado para usuarios finales)

**La forma más fácil** - un solo comando que instala todo:

```bash
sudo dpkg -i dist/navtunnel_1.0.0_amd64.deb
```

Si aparecen errores de dependencias:
```bash
sudo apt-get install -f
```

**Ventajas:**
- ✅ Instala todas las dependencias automáticamente
- ✅ Configura permisos sudo automáticamente (no necesitarás usar sudo para ejecutar la app)
- ✅ Crea entrada en el menú de aplicaciones
- ✅ Instala icono del sistema
- ✅ Desinstalación limpia con `sudo apt remove navtunnel`

Después de instalar, busca "NavTunnel" en tu menú de aplicaciones.

### Opción 2: Compilar con Docker (Para desarrolladores - NO requiere Go instalado)

**Ventaja:** No necesitas instalar Go ni dependencias de desarrollo en tu PC.

```bash
# Con Taskfile
task build-docker

# O con script dev.sh
./dev.sh build-binary

# El binario estará en dist/navtunnel
./dist/navtunnel
```

Ver [BUILD.md](BUILD.md) para documentación completa de compilación.

### Opción 3: Compilar desde el código fuente (requiere Go)

1. **Clonar el repositorio**
   ```bash
   git clone <repo-url>
   cd binariovpnprey
   ```

2. **Verificar dependencias**
   ```bash
   make check-deps
   ```

3. **Compilar el binario**
   ```bash
   make build
   ```

   El binario se generará en `bin/navtunnel`

4. **Instalar en el sistema** (opcional)
   ```bash
   make install
   ```

   Esto copiará el binario a `/usr/local/bin/navtunnel`

## Primer Uso

### Configuración Inicial (muy simple)

1. **Lanzar la aplicación**
   - Si instalaste con .deb: busca "NavTunnel" en el menú de aplicaciones
   - Si compilaste: ejecuta `./dist/navtunnel` o `navtunnel` si está en PATH

2. **Seleccionar archivo .ovpn**
   - En el primer inicio, aparecerá un diálogo de bienvenida
   - Haz clic en "Seleccionar Archivo VPN"
   - Navega hasta tu archivo `.ovpn` y selecciónalo
   - La aplicación guardará esta configuración automáticamente en `~/.config/NavTunnel/config.json`

3. **Cambiar archivo VPN** (opcional)
   - Si necesitas cambiar el archivo .ovpn más tarde, usa el botón "Cambiar archivo VPN" en la ventana principal

**Nota:** Ya no necesitas crear directorios manualmente ni renombrar archivos. La aplicación lo maneja todo.

## Uso Diario

### Conectar a la VPN

1. **Ejecutar NavTunnel**
   - Desde el menú de aplicaciones (si usaste .deb)
   - O ejecuta `navtunnel` desde terminal

2. **Presionar Conectar**
   - La aplicación se minimizará al system tray (icono en la barra del sistema)
   - **No necesitas usar sudo** - los permisos se configuraron automáticamente con el .deb

3. **Autenticación**
   - **Paso 1**: Ingresa tu usuario corporativo
   - **Paso 2**: Ingresa tu contraseña
   - **Paso 3**: Ingresa tu código OTP de 6 dígitos

4. **Conexión establecida**
   - El icono del system tray cambiará a verde ✅
   - Verás "Conexión establecida" en los logs
   - La ventana se puede minimizar (va al tray)

### System Tray

**Iconos de estado:**
- 🔘 **Gris**: Desconectado
- 🟠 **Naranja**: Conectando/Autenticando
- 🟢 **Verde**: Conectado exitosamente
- 🔴 **Rojo**: Error de conexión

**Menú del tray** (clic derecho en el icono):
- **Estado**: Muestra el estado actual
- **Conectar**: Inicia la conexión
- **Desconectar**: Cierra la conexión
- **Mostrar ventana**: Abre la ventana principal
- **Salir**: Cierra completamente la aplicación

**Minimizar a tray:**
- Al cerrar la ventana (X), la app **NO se cierra**
- Se minimiza al system tray y sigue funcionando
- Para cerrar completamente: usa "Salir" del menú del tray

### Desconectar

- **Opción 1**: Presiona "Desconectar" en la ventana principal
- **Opción 2**: Usa "Desconectar" en el menú del system tray

### Manejo de errores

- **Contraseña incorrecta**: Se te pedirá ingresar solo la contraseña nuevamente
- **OTP inválido/expirado**: Se te pedirá ingresar solo el OTP nuevamente
- **Archivo .ovpn no válido**: Usa el botón "Cambiar archivo VPN" para seleccionar otro

## Estructura del Proyecto

```
.
├── cmd/
│   └── navtunnel/
│       └── main.go                    # Punto de entrada
├── internal/
│   ├── core/
│   │   ├── openvpn.go                # Gestión del proceso (usa platform abstraction)
│   │   └── manager.go                # Management Interface parser
│   ├── platform/                      # ⭐ Abstracciones multi-plataforma
│   │   ├── platform.go               # Interface común
│   │   ├── platform_*.go             # Build tags por plataforma
│   │   ├── linux/linux.go            # Implementación Linux (completa)
│   │   ├── windows/windows.go        # Implementación Windows (stub)
│   │   └── darwin/darwin.go          # Implementación macOS (stub)
│   ├── tray/                          # ⭐ System tray abstraction
│   │   ├── tray.go                   # Interface común (permite migrar a AppIndicator3)
│   │   ├── systray.go                # Implementación con getlantern/systray
│   │   └── icons/                    # Iconos de estado (PNG)
│   ├── config/                        # ⭐ Configuración persistente
│   │   └── config.go                 # Gestión de configuración JSON
│   ├── ui/
│   │   ├── app.go                    # Ventana principal (Fyne - cross-platform)
│   │   └── prompts.go                # Modales de entrada
│   └── logs/
│       └── buffer.go                 # Buffer circular de logs
├── packaging/                         # ⭐ Packaging para distribución
│   ├── build-deb.sh                  # Script de construcción .deb
│   ├── create-icon.py                # Generador de icono
│   └── debian/                       # Estructura del paquete .deb
│       ├── DEBIAN/
│       │   ├── control               # Metadata y dependencias
│       │   ├── postinst              # Configura sudo automáticamente
│       │   └── prerm                 # Limpieza en desinstalación
│       └── usr/
│           ├── bin/                  # Binario instalado
│           └── share/
│               ├── applications/     # Desktop entry
│               └── icons/            # Iconos del sistema
├── build/                             # Scripts de build por plataforma
├── dist/                              # Binarios compilados y paquetes .deb
├── configs/                           # Configuraciones específicas por OS
├── go.mod                             # Dependencias
├── Makefile                           # Build system multi-plataforma
├── README.md                          # Este archivo
└── ARCHITECTURE.md                    # 📖 Documentación de arquitectura
```

> Para más detalles sobre la arquitectura multi-plataforma, consulta [ARCHITECTURE.md](ARCHITECTURE.md)

## Desarrollo

### Opción 1: Desarrollo con Docker (Recomendado)

Para un entorno de desarrollo reproducible con hot-reload:

```bash
# Setup inicial (primera vez)
./dev.sh setup

# O si tienes Task instalado
task setup

# Iniciar desarrollo con hot-reload
./dev.sh dev
# O
task dev
```

**Ventajas del entorno Docker:**
- ✅ Hot-reload automático al editar archivos
- ✅ Dependencias pre-instaladas
- ✅ Entorno reproducible
- ✅ No contamina tu sistema host
- ✅ Fácil limpieza

Ver [DOCKER-README.md](DOCKER-README.md) para documentación completa.

### Opción 2: Desarrollo Local (Tradicional)

#### Comandos Make disponibles

#### Desarrollo Local
```bash
make build          # Compilar para la plataforma actual
make build-release  # Compilar optimizado para distribución
make run            # Compilar y ejecutar
make clean          # Limpiar archivos generados
make deps           # Instalar dependencias de Go
```

#### Multi-Plataforma
```bash
make build-all      # Compilar para todas las plataformas (arquitectura principal)
make build-all-arch # Compilar para todas las plataformas y arquitecturas

# Builds específicos
make build-linux    # Linux amd64
make build-windows  # Windows amd64
make build-darwin   # macOS amd64 + arm64 (Apple Silicon)
```

#### Utilidades
```bash
make install        # Instalar en /usr/local/bin (Linux/macOS)
make uninstall      # Desinstalar del sistema
make check-deps     # Verificar dependencias del sistema
make setup-config   # Crear directorio ~/NavTunnel
make info           # Mostrar información del sistema
make help           # Mostrar ayuda completa
```

Los binarios multi-plataforma se generan en `dist/<os>-<arch>/`

### Compilar para distribución

Para generar un binario optimizado sin símbolos de debug:

```bash
make build-release
```

El binario resultante en `bin/navtunnel` será más pequeño y estará listo para distribuir.

## Seguridad

- **No almacena credenciales**: Todas las credenciales se solicitan en tiempo real
- **--auth-nocache**: OpenVPN no cachea credenciales
- **Logs sanitizados**: No se imprimen contraseñas ni OTPs en los logs
- **Elevación puntual**: Solo se solicitan permisos de root cuando es necesario

## Troubleshooting

### Error: "OpenVPN no está instalado"
```bash
sudo apt install openvpn
```

### Error: "pkexec no está disponible"
```bash
sudo apt install policykit-1
```

### Error: "No se encontró el archivo de configuración"
- Verifica que el archivo existe: `ls -la ~/NavTunnel/tu-archivo.ovpn`
- Verifica los permisos: `chmod 644 ~/NavTunnel/tu-archivo.ovpn`

### Error: "--up script fails with '/etc/openvpn/update-systemd-resolved'"
**Causa:** Tu archivo .ovpn requiere el script `update-systemd-resolved` que no está instalado.

**Solución:**
```bash
sudo apt install openvpn-systemd-resolved
```

**Nota:** Si instalas NavTunnel v1.0.0 o superior con el .deb, esta dependencia se instala automáticamente. Si el error persiste después de reinstalar, verifica que el script existe:
```bash
ls -l /etc/openvpn/update-systemd-resolved
```

### OTP siempre falla
- Verifica que la hora de tu sistema esté sincronizada:
  ```bash
  sudo apt install ntpdate
  sudo ntpdate pool.ntp.org
  ```

### No puedo compilar (error con Fyne)
```bash
sudo apt install libgl1-mesa-dev xorg-dev
```

## Creación del Paquete .deb

Si eres desarrollador y quieres reconstruir el paquete .deb:

```bash
# 1. Compilar el binario primero
./dev.sh build-binary
# O
task build-docker

# 2. Crear el icono (requiere Python + PIL)
cd packaging
python3 create-icon.py
cd ..

# 3. Construir el paquete .deb
cd packaging
./build-deb.sh

# El paquete estará en dist/navtunnel_1.0.0_amd64.deb
```

Ver [packaging/debian/DEBIAN/control](packaging/debian/DEBIAN/control) para la lista completa de dependencias.

## Limitaciones y Roadmap

### ✅ v1.0 Actual (Linux - Completo)

Características implementadas:

- ✅ Soporta Linux/Ubuntu completo con GUI
- ✅ System tray con iconos de estado
- ✅ File picker visual (selección gráfica de .ovpn)
- ✅ Configuración persistente (~/.config/NavTunnel/)
- ✅ Packaging .deb con configuración automática de permisos
- ✅ Instalación desde menú de aplicaciones
- ✅ Minimizar a tray en lugar de cerrar
- ✅ Multi-factor authentication (usuario + password + OTP)

Limitaciones actuales:

- Solo soporta Linux (Windows y macOS en stubs)
- No recuerda credenciales entre sesiones (por seguridad)
- No tiene auto-reconexión
- System tray usa getlantern/systray (ver próximos pasos para AppIndicator3)

### 🔄 Próximos Pasos

#### Mejoras de System Tray (Corto Plazo)

**Migración a AppIndicator3** para mejor integración con GNOME:

La arquitectura actual en `internal/tray/` está preparada para esto:

```go
// internal/tray/tray.go - Interface común
type TrayIcon interface {
    SetTitle(title string)
    SetIcon(iconType IconType)
    SetTooltip(text string)
    Run(onReady func(), onExit func())
    Quit()
}

// Cambiar implementación sin tocar el resto del código:
// internal/tray/tray.go
func New(callbacks MenuCallbacks) TrayIcon {
    return NewAppIndicator(callbacks)  // En lugar de NewSystray()
}
```

**Pasos para implementar AppIndicator3:**

1. Crear `internal/tray/appindicator.go` con implementación nativa
2. Usar CGo con `libayatana-appindicator3` directamente
3. Mejor integración con GNOME Shell y Unity
4. Soporte para menús más complejos y notificaciones nativas

#### Windows Support (v1.1)

- [ ] Implementar `internal/platform/windows/` completo
- [ ] System tray nativo de Windows
- [ ] Elevación con UAC
- [ ] Instalador .msi con WiX
- [ ] Firmar código para Windows Defender

#### macOS Support (v1.2)

- [ ] Implementar `internal/platform/darwin/` completo
- [ ] System tray para macOS (NSStatusBar)
- [ ] Elevación con osascript/SMJobBless
- [ ] Bundle .app y .dmg
- [ ] Soporte completo para Apple Silicon (arm64)
- [ ] Firmar y notarizar para Gatekeeper

#### Features Adicionales (v1.3+)

- [ ] Soporte de múltiples perfiles VPN
- [ ] Recordar usuario (con keyring/Credential Manager/Keychain)
- [ ] Auto-reconexión con backoff exponencial
- [ ] Reglas polkit por grupo (sin prompt de contraseña)
- [ ] Auto-update system
- [ ] Logging configurable con niveles
- [ ] Estadísticas de uso (tiempo conectado, datos transferidos)

Para más detalles sobre la arquitectura y cómo implementar estas características, consulta [ARCHITECTURE.md](ARCHITECTURE.md)

## Soporte

Para reportar problemas o solicitar características, abre un issue en el repositorio:
https://github.com/lavp2393/client-vpn/issues

## Licencia

Este proyecto está licenciado bajo la Licencia MIT - ver el archivo [LICENSE](LICENSE) para más detalles.

Copyright (c) 2025 Luis Alejandro Vazquez
