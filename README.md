# PreyVPN - Cliente OpenVPN con GUI Multi-Plataforma

**Version 1.0.0 - Stable Release (Linux/Ubuntu - 2025-11-07)**

Cliente OpenVPN con interfaz gráfica que facilita la conexión a la VPN corporativa mediante autenticación multi-factor (usuario + contraseña + OTP).

## Estado del Proyecto

| Plataforma | Estado | Arquitecturas |
|------------|--------|---------------|
| **Linux** | ✅ MVP Completo | amd64, arm64 |
| **Windows** | 🚧 En desarrollo | amd64, arm64 |
| **macOS** | 🚧 En desarrollo | amd64, arm64 |

> **Nota:** La implementación actual está enfocada en Linux/Ubuntu. El código está estructurado para soportar múltiples plataformas mediante abstracciones, con stubs preparados para Windows y macOS.

## Características

- **Interfaz gráfica simple**: Sin necesidad de usar la terminal
- **Autenticación multi-factor**: Usuario → Contraseña → OTP (LinOTP)
- **Gestión automática**: Maneja toda la comunicación con OpenVPN
- **Logs en vivo**: Visualización de eventos de conexión
- **Seguro**: No almacena credenciales (--auth-nocache)

## Requisitos del Sistema

### Sistema Operativo
- Ubuntu Desktop 20.04 o superior
- Otras distribuciones basadas en Debian (pueden funcionar)

### Dependencias

1. **OpenVPN**
   ```bash
   sudo apt install openvpn
   ```

2. **PolicyKit** (para elevación de privilegios)
   ```bash
   sudo apt install policykit-1
   ```

3. **Go 1.21+** (solo para compilar)
   ```bash
   # Descargar desde https://golang.org/dl/
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin
   ```

4. **Dependencias de GUI** (para Fyne)
   ```bash
   sudo apt install libgl1-mesa-dev xorg-dev
   ```

## Instalación

### Opción 1: Compilar con Docker (Recomendado - NO requiere Go instalado)

**Ventaja:** No necesitas instalar Go ni dependencias de desarrollo en tu PC.

```bash
# Con Taskfile
task build-docker

# O con script dev.sh
./dev.sh build-binary

# El binario estará en dist/preyvpn
./dist/preyvpn
```

Ver [BUILD.md](BUILD.md) para documentación completa de compilación.

### Opción 2: Compilar desde el código fuente (requiere Go)

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

   El binario se generará en `bin/preyvpn`

4. **Instalar en el sistema** (opcional)
   ```bash
   make install
   ```

   Esto copiará el binario a `/usr/local/bin/preyvpn`

### Opción 2: Usar binario pre-compilado

Si recibes un binario ya compilado:

1. **Darle permisos de ejecución**
   ```bash
   chmod +x preyvpn
   ```

2. **Moverlo a un directorio en PATH** (opcional)
   ```bash
   sudo mv preyvpn /usr/local/bin/
   ```

## Configuración

1. **Crear el directorio de configuración**
   ```bash
   mkdir -p ~/PreyVPN
   ```

   O usar el comando make:
   ```bash
   make setup-config
   ```

2. **Colocar el archivo de configuración VPN**

   Copia el archivo `.ovpn` que te proporciona tu organización:
   ```bash
   cp /ruta/a/tu/archivo.ovpn ~/PreyVPN/prey-prod.ovpn
   ```

   **Importante**: El archivo DEBE llamarse exactamente `prey-prod.ovpn`

## Uso

### Ejecutar la aplicación

Si instalaste con `make install`:
```bash
preyvpn
```

Si no instalaste, desde el directorio del proyecto:
```bash
./bin/preyvpn
```

### Flujo de conexión

1. **Abrir la aplicación**
   - La aplicación verificará si existe el archivo de configuración
   - Si no existe, mostrará instrucciones

2. **Conectar**
   - Presiona el botón "Conectar"
   - Se te pedirá tu contraseña de administrador (para `pkexec`)

3. **Autenticación**
   - **Paso 1**: Ingresa tu usuario corporativo
   - **Paso 2**: Ingresa tu contraseña
   - **Paso 3**: Ingresa tu código OTP de 6 dígitos

4. **Conectado**
   - Verás el mensaje "Conexión establecida ✅"
   - Los logs mostrarán los eventos de conexión

5. **Desconectar**
   - Presiona el botón "Desconectar"
   - La conexión se cerrará limpiamente

### Manejo de errores

- **Contraseña incorrecta**: Se te pedirá ingresar solo la contraseña nuevamente
- **OTP inválido/expirado**: Se te pedirá ingresar solo el OTP nuevamente
- **Archivo de configuración no encontrado**: Verifica que `~/PreyVPN/prey-prod.ovpn` existe

## Estructura del Proyecto

```
.
├── cmd/
│   └── preyvpn/
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
│   ├── ui/
│   │   ├── app.go                    # Ventana principal (Fyne - cross-platform)
│   │   └── prompts.go                # Modales de entrada
│   └── logs/
│       └── buffer.go                 # Buffer circular de logs
├── build/                             # Scripts de build por plataforma
├── dist/                              # Binarios compilados multi-plataforma
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
make setup-config   # Crear directorio ~/PreyVPN
make info           # Mostrar información del sistema
make help           # Mostrar ayuda completa
```

Los binarios multi-plataforma se generan en `dist/<os>-<arch>/`

### Compilar para distribución

Para generar un binario optimizado sin símbolos de debug:

```bash
make build-release
```

El binario resultante en `bin/preyvpn` será más pequeño y estará listo para distribuir.

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
- Verifica que el archivo existe: `ls -la ~/PreyVPN/prey-prod.ovpn`
- Verifica los permisos: `chmod 644 ~/PreyVPN/prey-prod.ovpn`

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

## Limitaciones y Roadmap

### MVP Actual (Linux)

Este es un MVP (Minimum Viable Product) enfocado en Linux con las siguientes limitaciones:

- ✅ Soporta Linux/Ubuntu completo
- 🚧 Windows y macOS en desarrollo (estructura lista, implementación pendiente)
- Solo un perfil VPN (prey-prod.ovpn)
- No recuerda el usuario entre sesiones
- No tiene auto-reconexión
- Packaging básico pendiente (.deb, .rpm, etc.)

### Próximas Versiones

**v0.2 - Windows Support**
- Implementación completa para Windows
- Elevación con UAC
- Instalador .msi

**v0.3 - macOS Support**
- Implementación completa para macOS
- Soporte para Apple Silicon
- Bundle .app y .dmg

**v1.0 - Feature Complete**
- Soporte de múltiples perfiles
- Recordar usuario (keyring)
- Auto-reconexión
- Packaging nativo para todas las plataformas

Para más detalles, consulta [ARCHITECTURE.md](ARCHITECTURE.md)

## Soporte

Para reportar problemas o solicitar características, contacta al equipo de desarrollo.

## Licencia

[Especificar licencia según tu organización]
