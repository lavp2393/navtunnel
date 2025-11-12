# PreyVPN - Contexto Técnico del Problema de Autenticación OTP

## Última actualización: 2025-10-29

---

## 1. Descripción del Problema

**Síntoma:** El cliente PreyVPN no está solicitando el código OTP durante el proceso de autenticación, lo que resulta en fallos de conexión con `AUTH_FAILED`.

**Comportamiento esperado:**
1. Usuario ingresa username (email corporativo)
2. Usuario ingresa password (contraseña LDAP/PAM)
3. Usuario ingresa OTP (código de 6 dígitos de LinOTP)
4. Conexión exitosa

**Comportamiento actual:**
- Usuario ingresa username ✅
- Sistema NO pide password ❌
- OpenVPN intenta conectar sin credenciales completas
- Falla con `AUTH_FAILED`

---

## 2. Arquitectura de Autenticación

### Stack de Autenticación del Servidor

```
Cliente OpenVPN
    ↓
Servidor OpenVPN (vpn.example.com:1194)
    ↓
PAM (Pluggable Authentication Modules)
    ├─→ pam_sss (System Security Services - LDAP)
    │   └─→ Valida: username + password
    │
    └─→ pam_linotp (LinOTP plugin)
        └─→ Valida: username + OTP
            └─→ LinOTP Server (https://otp.example.com/validate/simplecheck)
                └─→ Recibe: realm=vpn&user=user@example.com&pass=<OTP>
```

### Log de Autenticación Exitosa (desde /var/log/auth.log)

```
pam_linotp[372481]: connecting to url:https://otp.example.com/validate/simplecheck
                     with parameters realm=vpn&user=user%40example.com&pass=123456
pam_linotp[372481]: result :-) User 'user@example.com' authenticated successfully
pam_sss(openvpn:auth): authentication success; user=user@example.com
```

**Clave:** LinOTP recibe `pass=123456` (solo el OTP de 6 dígitos, NO el password real).

---

## 3. Configuración OpenVPN

### Archivo: `~/PreyVPN/prey-prod.ovpn`

```
client
dev tun
proto udp
remote vpn.example.com 1194
auth-user-pass
static-challenge "Your OTP" 1    ← Indica que hay un challenge estático (OTP)
auth-retry interact
remote-cert-tls server
verb 4
```

**Parámetros críticos:**
- `static-challenge "Your OTP" 1`:
  - `"Your OTP"` = Texto del prompt
  - `1` = Echo mode (mostrar caracteres al escribir)
  - Indica autenticación de 3 factores: username + password + OTP

---

## 4. OpenVPN Management Interface Protocol

### Comandos del Cliente PreyVPN

```bash
# Al iniciar OpenVPN
pkexec openvpn \
  --config ~/PreyVPN/prey-prod.ovpn \
  --management 127.0.0.1 <PORT> \
  --management-query-passwords \
  --management-hold \
  --auth-retry interact \
  --auth-nocache \
  --verb 4
```

### Flujo del Management Interface (Esperado)

```
1. OpenVPN → Cliente: >PASSWORD:Need 'Auth' username/password SC:1,Your OTP
2. Cliente → OpenVPN: username "Auth" "user@example.com"
3. OpenVPN → Cliente: SUCCESS: 'Auth' username entered, but not yet verified
4. OpenVPN → Cliente: >PASSWORD:Need 'Auth' password
5. Cliente → OpenVPN: password "Auth" "<password_real>"
6. OpenVPN → Cliente: SUCCESS: 'Auth' password entered, but not yet verified
7. OpenVPN → Cliente: >PASSWORD:Need 'Auth' password (SC response)
8. Cliente → OpenVPN: password "Auth" "654321"
9. OpenVPN valida con PAM/LinOTP
10. OpenVPN → Cliente: >STATE:*,CONNECTED,SUCCESS,*
```

### Flujo Actual (Problemático)

```
1. OpenVPN → Cliente: >PASSWORD:Need 'Auth' username/password SC:1,Your OTP
2. Cliente → OpenVPN: username "Auth" "user@example.com"
3. OpenVPN → Cliente: SUCCESS: 'Auth' username entered, but not yet verified
4. ⚠️ OpenVPN envía: SENT CONTROL [ServerName]: 'PUSH_REQUEST' (status=1)
   └─→ Esto indica que OpenVPN piensa que la auth está completa
5. OpenVPN → Cliente: AUTH: Received control message: AUTH_FAILED
6. OpenVPN → Cliente: >PASSWORD:Verification Failed: 'Auth'
7. OpenVPN reinicia: SIGUSR1[soft,auth-failure] received
8. OpenVPN → Cliente: >HOLD:Waiting for hold release:1
```

**Problema identificado:** OpenVPN intenta hacer `PUSH_REQUEST` (paso 4) sin haber recibido el password ni el OTP.

---

## 5. Implementación Actual del Cliente

### Estructura del Código

```
internal/
├── core/
│   ├── openvpn.go          # Gestión del proceso (pkexec, pipes)
│   └── manager.go          # Management Interface parser
├── ui/
│   ├── app.go              # UI principal (Fyne)
│   └── prompts.go          # Modales de autenticación
└── logs/
    └── buffer.go           # Buffer circular de logs
```

### Lógica de Detección de Prompts (manager.go)

```go
// Detectar prompt inicial con static-challenge
if strings.HasPrefix(line, ">PASSWORD:") {
    if strings.Contains(line, "SC:") {
        // Static challenge detectado
    }
    handlePasswordPrompt(line)
}

// Después de enviar username exitosamente
if strings.Contains(line, "SUCCESS") && strings.Contains(line, "username entered") {
    // Emitir evento para pedir password
    m.events <- Event{Type: EventAskPass}
}

// Después de enviar password exitosamente
if m.passwordSent && strings.HasPrefix(line, ">PASSWORD:") {
    // Emitir evento para pedir OTP
    m.events <- Event{Type: EventAskOTP}
}
```

### Formato de Comandos (con correcciones aplicadas)

```go
// Username
username "Auth" "user@example.com"

// Password (con escape de caracteres especiales)
password "Auth" "MyP@ssw0rd!123"
                 └─ Entrecomillado para manejar: espacios, @, !, #, $, etc.

// OTP
password "Auth" "789012"
```

---

## 6. Correcciones Aplicadas (Historial)

### Iteración 1: Corrección del comando OTP
- **Problema:** Se enviaba `username "SCR:Auth" <otp>` ❌
- **Solución:** Cambiar a `password "Auth" <otp>` ✅

### Iteración 2: Timing del prompt OTP
- **Problema:** Se pedía OTP inmediatamente después del password sin esperar respuesta de OpenVPN
- **Solución:** Esperar a que OpenVPN envíe el segundo `>PASSWORD:` prompt

### Iteración 3: Entrecomillado de credenciales
- **Problema:** Passwords con espacios/caracteres especiales no funcionaban
- **Solución:**
  - Entrecomillar todas las credenciales: `password "Auth" "<value>"`
  - Escapar `\` y `"` dentro de las credenciales

### Iteración 4: Detección de errores
- **Problema:** Se confundía `>PASSWORD:Verification Failed` con un prompt de OTP
- **Solución:** Detectar "Verification Failed" y no pedir credenciales en ese caso

### Iteración 5: Reset de estado
- **Problema:** Después de `AUTH_FAILED`, el estado `passwordSent` permanecía en `true`
- **Solución:** Resetear estado cuando se detecta `>HOLD:` (OpenVPN reiniciando)

---

## 7. Problema Actual No Resuelto

### Observación Clave de los Logs

```
SUCCESS: 'Auth' username entered, but not yet verified
[NO SE VE NINGÚN PROMPT DE PASSWORD AQUÍ]
SENT CONTROL [PreyINC]: 'PUSH_REQUEST' (status=1)
AUTH_FAILED
```

**Hipótesis A:** El prompt de password SÍ fue enviado por OpenVPN pero no fue detectado por el parser.

**Hipótesis B:** OpenVPN NO está enviando el prompt de password porque espera que se envíe de forma diferente (posiblemente combinado con el username cuando hay static-challenge).

**Hipótesis C:** El servidor está configurado de forma diferente y NO usa el flujo estándar de 3 pasos.

---

## 8. Comandos de Debugging

### Ver logs del servidor OpenVPN
```bash
ssh user@vpn.example.com
sudo tail -f /var/log/auth.log
```

### Probar conexión manual con OpenVPN CLI
```bash
sudo openvpn --config ~/PreyVPN/prey-prod.ovpn --verb 4
```

**Flujo esperado:**
```
Enter Auth Username: user@example.com
Enter Auth Password: <password_real>
CHALLENGE: Your OTP 345678
Response: 345678
```

### Probar con Management Interface manual
```bash
# Terminal 1: Iniciar OpenVPN con management
sudo openvpn --config prey-prod.ovpn \
  --management 127.0.0.1 7505 \
  --management-query-passwords \
  --management-hold

# Terminal 2: Conectar al management interface
telnet 127.0.0.1 7505

# Enviar comandos:
hold release
# Esperar prompt >PASSWORD:...
username "Auth" "user@example.com"
# Esperar SUCCESS y siguiente prompt
password "Auth" "<password>"
# Esperar siguiente prompt
password "Auth" "901234"
```

---

## 9. Posibles Soluciones a Explorar

### Opción 1: Forzar el flujo de 3 pasos
Modificar el parser para que después del `SUCCESS` del username, **siempre** emita `EventAskPass`, incluso si OpenVPN no envía explícitamente el prompt.

### Opción 2: Enviar password + OTP combinados
Algunos servidores con static-challenge esperan:
```
password "Auth" "<password>\n<otp>"
```
O con CRV1 encoding:
```
password "Auth" "CRV1::<flags>:<state_id>::<password>:<otp>"
```

### Opción 3: Usar --auth-user-pass con archivo
Crear un archivo temporal con:
```
username
password
```
Y dejar que OpenVPN maneje el OTP interactivamente.

### Opción 4: Script wrapper con expect
Usar `expect` para automatizar la interacción con OpenVPN CLI:
```tcl
spawn openvpn --config prey-prod.ovpn
expect "Enter Auth Username:"
send "user@example.com\r"
expect "Enter Auth Password:"
send "$password\r"
expect "CHALLENGE: Your OTP"
send "$otp\r"
```

---

## 10. Información de Contacto y Referencias

### Documentación OpenVPN
- Management Interface: https://openvpn.net/community-resources/management-interface/
- Static Challenge: https://community.openvpn.net/openvpn/wiki/StaticChallengeResponse

### LinOTP
- Documentación: https://www.linotp.org/doc/latest/
- PAM plugin: https://github.com/LinOTP/linotp-auth-pam

### Logs del Cliente (PreyVPN)
Los logs se muestran en la ventana de la aplicación y contienen:
- `[MGMT]` - Mensajes del management interface
- `[DEBUG]` - Logs de debugging internos
- `[DETECTED]` - Eventos detectados
- `[OpenVPN stdout]` - Salida de OpenVPN

---

## 11. Próximos Pasos Recomendados

1. **Capturar el flujo completo del management interface:**
   - Usar `telnet` para conectar manualmente al management interface
   - Documentar EXACTAMENTE qué prompts envía el servidor después del username

2. **Verificar configuración del servidor:**
   - Revisar `/etc/openvpn/server.conf` en el servidor
   - Confirmar que `static-challenge` está configurado correctamente

3. **Probar con OpenVPN CLI:**
   - Confirmar que la conexión funciona manualmente
   - Capturar el flujo exacto de prompts

4. **Considerar alternativas:**
   - Si el management interface no funciona correctamente, usar un wrapper con `expect`
   - O usar `pexpect` en Python para mayor control

---

## 12. Estado Actual del Código

### Archivos Modificados
- `internal/core/manager.go` - Múltiples correcciones al parser y comandos
- `internal/ui/app.go` - Sin cambios recientes
- `internal/ui/prompts.go` - Sin cambios recientes

### Últimas Correcciones (2025-10-29)
- ✅ Entrecomillado de credenciales
- ✅ Escape de caracteres especiales (`\`, `"`)
- ✅ Detección de mensajes de error vs prompts
- ✅ Reset de estado al reiniciar OpenVPN
- ❌ NO RESUELTO: Prompt de password no aparece después del username

---

## 13. Notas Importantes

### Seguridad
- Las credenciales NO se cachean (`--auth-nocache`)
- Los logs sanitizan las credenciales (solo muestran primeros/últimos 2 caracteres)
- El management interface es local (127.0.0.1), no expuesto a la red

### Passwords Complejos
El código actual maneja correctamente:
- Espacios: `My Password 123`
- Caracteres especiales: `P@ssw0rd!#$%`
- Backslashes: `Pass\word`
- Comillas: `Pass"word`

Todos son escapados y entrecomillados correctamente.

---

**Última sesión:** 2025-10-29 20:30 UTC
**Estado:** Problema no resuelto - requiere debugging adicional del management interface
