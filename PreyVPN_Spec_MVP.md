# PreyVPN — Especificación MVP v1.0 (Ubuntu, con .deb) ✅ COMPLETO

## 1) Objetivo
Binario **Go** con GUI completa diseñada para usuarios no técnicos que:
- Lanza **OpenVPN** usando cualquier perfil .ovpn seleccionado visualmente
- Gestiona prompts **usuario → contraseña → OTP** vía **Management Interface**
- Permite **Conectar/Desconectar** 100% desde GUI (cero terminal)
- Incluye **system tray** con iconos de estado
- Se instala con **.deb** que configura dependencias y permisos automáticamente
- **No** cambia nada del backend (OpenVPN + PAM/LDAP + LinOTP)

---

## 2) Supuestos del sistema
- Sistema: **Ubuntu Desktop**.
- `openvpn` instalado y disponible en `/usr/sbin/openvpn`, `/usr/bin/openvpn` o `$PATH`.
- `pkexec` disponible para elevación puntual.
- Perfil `.ovpn` provisto por la organización.

---

## 3) Perfil VPN (✅ ACTUALIZADO: File Picker Visual)
- **YA NO usa ruta fija** - el usuario selecciona el archivo visualmente
- Al primer inicio: aparece **diálogo de bienvenida** con botón "Seleccionar Archivo VPN"
- Se abre un **file picker visual** que filtra archivos `.ovpn`
- El usuario navega y selecciona cualquier archivo `.ovpn` (sin importar el nombre o ubicación)
- La ruta seleccionada se guarda en: `~/.config/PreyVPN/config.json`

**Beneficios:**
- ✅ 100% visual, no requiere terminal
- ✅ Funciona con cualquier archivo .ovpn
- ✅ No necesita renombrar archivos
- ✅ Configuración persistente entre sesiones
- ✅ Botón "Cambiar archivo VPN" para seleccionar otro perfil

---

## 4) Comportamiento del binario

### 4.1 Ciclo básico
1. **Inicio**
   - Verifica existencia de `~/PreyVPN/prey-prod.ovpn`.
   - Si no está: pantalla "perfil no encontrado" + **Reintentar**.
   - Si está: habilita **Conectar**.

2. **Conectar**
   - Selecciona **puerto de management** libre (ej. 49152–65535).
   - Lanza `openvpn` con **elevación** usando `pkexec`:
     ```bash
     openvpn --config ~/PreyVPN/prey-prod.ovpn        --management 127.0.0.1:<PORT> stdin        --auth-retry interact        --auth-nocache
     ```
   - Abre socket TCP a `127.0.0.1:<PORT>` y comienza a **parsear eventos**.

3. **Autenticación (prompts)**
   - Prompt 1 (usuario): `>PASSWORD:Need 'Auth' username`
   - Prompt 2 (contraseña): `>PASSWORD:Need 'Auth' pass`
   - Prompt 3 (OTP): cualquier `>PASSWORD:Need ...` posterior **o** presencia de `CHALLENGE/CRV1`
   - Respuestas (Management):
     - `username "Auth" <valor>`
     - `password "Auth" <valor>`
     - OTP con el **mismo tag** que solicite (normalmente `"Auth"`).

4. **Estados**
   - Mostrar **Conectando → Autenticando → Conectado**.
   - Conectado si aparece `>STATE:*,CONNECTED,SUCCESS,`.
   - Fallos:
     - `>STATE:*,AUTH_FAILED,` → mapear según etapa (pass/OTP).
     - `>FATAL:` → error de conexión.

5. **Desconectar**
   - Enviar señal para terminar el proceso `openvpn`.
   - Cerrar socket de management y volver a estado inicial.

---

## 5) UI Completa (✅ IMPLEMENTADO con Fyne)
### **Ventana principal**
  - Estado: "Conectado / Desconectado / Conectando..."
  - Indicador visual del archivo .ovpn configurado
  - Botones:
    - **Conectar / Desconectar**
    - **Cambiar archivo VPN** (abre file picker)
  - Log en vivo (buffer circular de últimas ~100 líneas; solo lectura)
  - **Minimizar a tray:** cerrar la ventana NO cierra la app, la minimiza al system tray

### **System Tray** ⭐ NUEVO
  - **Icono persistente** en la barra del sistema
  - **Estados visuales:**
    - 🔘 Gris: Desconectado
    - 🟠 Naranja: Conectando/Autenticando
    - 🟢 Verde: Conectado
    - 🔴 Rojo: Error
  - **Menú contextual** (clic derecho):
    - Estado actual
    - Conectar / Desconectar
    - Mostrar ventana
    - Salir (cierra completamente)
  - Implementado con abstracción (`internal/tray/`) que permite migrar a AppIndicator3 nativo

### **File Picker Visual** ⭐ NUEVO
  - Diálogo nativo de Fyne
  - Filtro automático para archivos `.ovpn`
  - Navegar por todo el sistema de archivos
  - Guarda la selección en `~/.config/PreyVPN/config.json`

### **Modales de Autenticación**
  - Usuario (placeholder: "usuario corporativo")
  - Contraseña (campo oculto)
  - OTP (6 dígitos; hint: "se renueva cada 30s")

### **Mensajes**
  - Archivo no configurado: "Selecciona tu archivo .ovpn para comenzar"
  - Contraseña incorrecta: `Contraseña incorrecta.`
  - OTP inválido/expirado: `OTP inválido o expirado.`
  - Conectado: `Conexión establecida ✅`
  - System tray: `Minimizado a la bandeja del sistema`

---

## 6) Seguridad
- **Obligatorio:** `--auth-nocache`.
- No persistir **contraseñas** ni **OTP**.
- (Opcional post-MVP) Recordar **solo** el usuario vía keyring del sistema.
- Logs sin secretos (no imprimir credenciales ni OTP).

---

## 7) Estructura de proyecto (✅ IMPLEMENTADA)

```
/cmd/preyvpn/main.go                    # Punto de entrada
/internal/
  /core/
    openvpn.go                          # spawn/kill de proceso con pkexec; usa platform abstraction
    manager.go                          # socket mgmt + parser + FSM (estados y eventos)
  /platform/                            # ⭐ Abstracciones multi-plataforma
    platform.go                         # Interface común
    platform_linux.go                   # Implementación Linux completa
    platform_windows.go                 # Stub (futuro)
    platform_darwin.go                  # Stub (futuro)
  /tray/                                # ⭐ System tray abstraction (NUEVO)
    tray.go                             # Interface TrayIcon común
    systray.go                          # Implementación con getlantern/systray
    /icons/                             # Iconos PNG de estado
      generate_icons.py
      disconnected.png, connecting.png, connected.png, error.png
  /config/                              # ⭐ Configuración persistente (NUEVO)
    config.go                           # Gestión de config.json en ~/.config/PreyVPN/
  /ui/
    app.go                              # ventana principal, file picker, system tray integration
    prompts.go                          # modales user/pass/otp
  /logs/
    buffer.go                           # buffer de log (rotación en memoria)
/packaging/                             # ⭐ Packaging .deb (NUEVO)
  build-deb.sh                          # Script de construcción del paquete
  create-icon.py                        # Generador de icono de la app
  /debian/
    /DEBIAN/
      control                           # Metadata y dependencias
      postinst                          # Configura /etc/sudoers.d/preyvpn
      prerm                             # Limpieza
    /usr/...                            # Estructura del paquete
```

**Contratos implementados:**
- `core.Start(configPath string, mgmtPort int) (events <-chan Event, send SendFns, stop func(), err error)`
- `type Event = AskUser | AskPass | AskOTP | Connected | AuthFailed{stage} | Fatal{reason} | LogLine{text}`
- `type SendFns struct { Username(v string); Password(v string); OTP(v string) }`
- `tray.TrayIcon` interface para abstracción del system tray
- `config.Config` struct para persistencia de configuración

---

## 8) Parsing de Management — patrones mínimos

### 8.1 Prompts (ejemplos reales orientativos)
```
>PASSWORD:Need 'Auth' username
>PASSWORD:Need 'Auth' pass
>PASSWORD:Need 'Auth' OTP
>INFO: CRV1:... <challenge-string> ...
```
**Regla simple:**  
- Si contiene `Need 'Auth'` y `username` → **AskUser**.  
- Si contiene `Need 'Auth'` y `pass` → **AskPass**.  
- Si aparece otro `Need` posterior **o** `CRV1`/`CHALLENGE` → **AskOTP**.

### 8.2 Éxito / fallo / fatal
```
>STATE:1730165123,CONNECTED,SUCCESS,10.8.0.10,xx.xx.xx.xx,,
>STATE:1730164999,AUTH_FAILED,,
>FATAL:Something bad happened
```

### 8.3 Envío de credenciales (formato)
```
username "Auth" myuser
password "Auth" mypassword
username "Auth" 123456        // si el servidor pide OTP como 'username' adicional, seguir el tag pedido
password "Auth" 123456        // o como 'password' adicional, según prompt
```
> **Nota:** usa exactamente el **tag** indicado en el prompt (normalmente `"Auth"`).

---

## 9) Elevación de privilegios
- Ejecutar OpenVPN con `pkexec` (GUI del sistema pedirá la contraseña si aplica).
- El binario debe:
  - Resolver ruta de `openvpn`.
  - Construir los argumentos.
  - Capturar PID del proceso hijo para **Desconectar** limpiamente.

---

## 10) Mapeo de errores (UX)
| Señal/Evento                         | Mensaje UI                    | Acción |
|-------------------------------------|-------------------------------|--------|
| `>STATE:*,AUTH_FAILED,` tras pass   | Contraseña incorrecta         | Re-pedir **solo** contraseña |
| `>STATE:*,AUTH_FAILED,` tras OTP    | OTP inválido o expirado       | Re-pedir **solo** OTP |
| Repetidos AUTH_FAILED al OTP        | Revisa la hora de tu equipo   | Mostrar hint de NTP |
| `>FATAL:` / timeouts                 | Error de conexión             | Permitir Reintentar |
| Falta `openvpn`                      | OpenVPN no está instalado     | Mostrar instrucción clara |

---

## 11) Criterios de aceptación (✅ TODOS CUMPLIDOS)
1. ✅ **Instalación con .deb:**
   - Instala todas las dependencias automáticamente
   - Configura permisos sudo sin intervención del usuario
   - Crea entrada en menú de aplicaciones
   - Instalable con un solo comando: `sudo dpkg -i preyvpn_1.0.0_amd64.deb`

2. ✅ **File Picker Visual:**
   - Al primer inicio, aparece diálogo de bienvenida
   - File picker filtra archivos `.ovpn` automáticamente
   - Guarda la selección en `~/.config/PreyVPN/config.json`
   - Botón "Cambiar archivo VPN" funciona correctamente

3. ✅ **Autenticación multi-factor:**
   - **Conectar** → aparecen **3 prompts** (usuario → contraseña → OTP) y termina en **Conectado**
   - Error de contraseña: muestra mensaje y re-pide **solo** contraseña
   - Error de OTP: muestra mensaje y re-pide **solo** OTP

4. ✅ **System Tray:**
   - Icono aparece en la barra del sistema
   - Cambia de color según estado (gris/naranja/verde/rojo)
   - Menú contextual con Connect/Disconnect/Show/Quit
   - Minimizar ventana → va al tray (no cierra la app)

5. ✅ **Desconectar:**
   - Mata el proceso OpenVPN limpiamente
   - Vuelve a estado inicial sin residuos

6. ✅ **Seguridad:**
   - `--auth-nocache` confirmado en logs de arranque
   - Logs sin secretos (no imprimen credenciales ni OTP)
   - Configuración sudo limitada solo a openvpn

7. ✅ **Experiencia de usuario:**
   - Cero uso de terminal requerido
   - No necesita editar archivos de configuración manualmente
   - Funciona sin sudo (permisos configurados automáticamente)

---

## 12) Características Completadas (v1.0)
- ✅ Arquitectura multi-plataforma con abstracciones
- ✅ Implementación completa para Linux/Ubuntu
- ✅ System tray con iconos de estado y menú contextual
- ✅ File picker visual (Fyne)
- ✅ Configuración persistente en `~/.config/PreyVPN/config.json`
- ✅ Packaging .deb con postinst/prerm scripts
- ✅ Desktop entry y menú de aplicaciones
- ✅ Multi-factor authentication (usuario + password + OTP)
- ✅ Minimizar a tray
- ✅ Abstracción del system tray (preparada para AppIndicator3)

---

## 13) Backlog (Próximas Versiones)

### v1.1 - System Tray Nativo
- [ ] Implementar `internal/tray/appindicator.go` con AppIndicator3 nativo (CGo)
- [ ] Mejor integración con GNOME Shell
- [ ] Notificaciones nativas del sistema
- [ ] Variable de entorno para elegir implementación

### v1.2 - Windows Support
- [ ] Implementar `internal/platform/windows/` completo
- [ ] System tray nativo de Windows
- [ ] Elevación con UAC
- [ ] Instalador .msi con WiX

### v1.3 - macOS Support
- [ ] Implementar `internal/platform/darwin/` completo
- [ ] System tray con NSStatusBar
- [ ] Elevación con osascript/SMJobBless
- [ ] Bundle .app y .dmg
- [ ] Firmar para Gatekeeper

### v2.0 - Features Avanzadas
- [ ] Recordar usuario con keyring/Credential Manager/Keychain
- [ ] Soporte de múltiples perfiles VPN con selector visual
- [ ] Auto-reconexión con backoff exponencial
- [ ] Regla polkit por grupo (sin prompt de password)
- [ ] Auto-update system
- [ ] Logging configurable con niveles
- [ ] Estadísticas de uso (tiempo conectado, datos)

---

## 13) Notas de implementación (prácticas)
- **Selección de puerto management:** intenta N aleatorios en rango 49152–65535 hasta éxito.
- **Lectura de management:** línea-a-línea; no bloqueante; emitir `LogLine` para todo.
- **Sanitización de logs:** nunca imprimir valores enviados en `username/password`.
- **Validación de comandos:** escapar/quote argumentos al invocar `pkexec` para evitar inyección.
- **Cierre limpio:** al desconectar, enviar SIGTERM al hijo y esperar; si no termina, SIGKILL con timeout.

---

**Fin del documento.**
