# PreyVPN - Guía de Uso

## 🚀 Instalación Recomendada (con .deb)

La forma más fácil de instalar PreyVPN es usar el paquete .deb:

```bash
sudo dpkg -i dist/preyvpn_1.0.0_amd64.deb

# Si hay errores de dependencias:
sudo apt-get install -f
```

**Ventajas de usar el .deb:**
- ✅ Instala todas las dependencias automáticamente
- ✅ Configura permisos de sudo automáticamente (no necesitarás password para openvpn)
- ✅ Crea entrada en el menú de aplicaciones
- ✅ Instala el icono del sistema
- ✅ Desinstalación limpia: `sudo apt remove preyvpn`

### Ejecutar después de instalar

**Desde el menú de aplicaciones:**
1. Presiona la tecla Super (Windows) o abre el menú de aplicaciones
2. Busca "PreyVPN"
3. Haz clic en el icono

**Desde terminal:**
```bash
preyvpn
```

### ⚠️ IMPORTANTE: NO uses `sudo`

```bash
# ❌ INCORRECTO
sudo preyvpn

# ✅ CORRECTO
preyvpn
```

**¿Por qué?**
- La aplicación GUI necesita acceso al D-Bus del usuario
- Ejecutar con `sudo` causa errores de permisos
- El paquete .deb ya configuró los permisos necesarios automáticamente

## 📍 Si compilaste sin instalar el .deb

Si solo compilaste el binario sin instalar el paquete:

```bash
# Ejecutar desde el directorio del proyecto
./dist/preyvpn
```

**Nota:** Si no instalaste el .deb, necesitarás usar `pkexec` para elevar privilegios cada vez que conectes (te pedirá contraseña de administrador).

## 🖼️ Características del System Tray

### Icono en la Barra del Sistema
- **Gris**: Desconectado
- **Naranja**: Conectando/Autenticando
- **Verde**: Conectado
- **Rojo**: Error

### Menú Contextual
Haz clic derecho en el icono para ver:
- Estado actual
- Conectar/Desconectar
- Abrir ventana
- Salir

### Minimizar a Tray
- Al cerrar la ventana (X), **no cierra la aplicación**
- La aplicación se minimiza al system tray
- Para cerrar completamente: usa "Salir" del menú del tray

## 📁 Configuración del Archivo VPN

### Primera vez (File Picker Visual)

**Ya NO necesitas crear directorios manualmente ni renombrar archivos.**

Al abrir PreyVPN por primera vez:

1. **Diálogo de bienvenida**
   - Aparece automáticamente si no tienes un archivo .ovpn configurado
   - Haz clic en "Seleccionar Archivo VPN"

2. **Seleccionar tu archivo .ovpn**
   - Se abre un explorador de archivos visual
   - Navega hasta donde guardaste tu archivo `.ovpn` (puede tener cualquier nombre)
   - Selecciona el archivo
   - La aplicación guarda esta configuración automáticamente

3. **Listo para conectar**
   - La aplicación recuerda tu archivo .ovpn entre sesiones
   - La configuración se guarda en: `~/.config/PreyVPN/config.json`

### Cambiar el archivo VPN

Si necesitas usar un archivo .ovpn diferente:

1. Abre PreyVPN
2. Haz clic en el botón **"Cambiar archivo VPN"** en la ventana principal
3. Selecciona el nuevo archivo .ovpn
4. ¡Listo! Ya puedes conectar con el nuevo perfil

## 🐛 Solución de Problemas

### Error: "Unable to connect to session D-Bus"
**Causa:** Ejecutaste la app con `sudo`
**Solución:** Ejecuta sin sudo:
```bash
# Si instalaste con .deb:
preyvpn

# Si solo compilaste:
./dist/preyvpn
```

### Error: "Se requiere contraseña de administrador" cada vez que conecto
**Causa:** No instalaste el paquete .deb (que configura permisos automáticamente)
**Solución:**
1. Instala el paquete .deb: `sudo dpkg -i dist/preyvpn_1.0.0_amd64.deb`
2. Verifica que se configuró: `cat /etc/sudoers.d/preyvpn`
3. Debería mostrar: `ALL ALL=(ALL) NOPASSWD: /usr/sbin/openvpn`

### Error: "pkexec no está disponible"
**Causa:** Falta PolicyKit
**Solución:**
```bash
sudo apt install policykit-1
# O reinstala con el .deb que instala dependencias automáticamente
sudo apt-get install -f
```

### Error: "OpenVPN no está instalado"
**Solución:**
```bash
sudo apt install openvpn
# O reinstala con el .deb que instala dependencias automáticamente
sudo apt-get install -f
```

### El icono del tray no aparece
**Causa:** Puede tardar unos segundos en inicializarse
**Solución:** Espera 2-3 segundos. Verás en los logs "System tray inicializado"

### PreyVPN no aparece en el menú de aplicaciones
**Causa:** No instalaste el paquete .deb
**Solución:** Instala con el .deb: `sudo dpkg -i dist/preyvpn_1.0.0_amd64.deb`

### No puedo seleccionar archivo .ovpn (el file picker no se abre)
**Causa:** Puede haber un problema con Fyne o el sistema de archivos
**Solución:**
1. Verifica que tienes permisos de lectura en el directorio del .ovpn
2. Intenta copiar el archivo a tu carpeta personal
3. Revisa los logs en la ventana de PreyVPN para más detalles

## 🔧 Desarrollo

### Compilar con Docker
```bash
./dev.sh build-binary
# O
task build-docker
```

### Desarrollo con hot-reload
```bash
task dev
```

## 📝 Logs

Los logs aparecen en la ventana principal en tiempo real:
- Eventos de conexión
- Mensajes de OpenVPN
- Estado del system tray
- Errores (si los hay)

## 🔑 Credenciales

- **Usuario y contraseña**: Se pueden recordar usando el keyring del sistema
- **OTP**: Nunca se guarda (por seguridad)
- Primera vez: marca la casilla "Recordar credenciales"

## ⌨️ Atajos y Tips

1. **Inicio rápido**: Copia el binario a `/usr/local/bin/` para ejecutarlo desde cualquier terminal
2. **Auto-inicio**: Configura PreyVPN para iniciarse con el sistema (ver sección siguiente)
3. **Múltiples ventanas**: La app solo permite una instancia a la vez

## 🚀 Auto-inicio (opcional)

Crear archivo `~/.config/autostart/preyvpn.desktop`:
```ini
[Desktop Entry]
Type=Application
Name=PreyVPN
Exec=/usr/local/bin/preyvpn
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
```

La aplicación iniciará minimizada en el tray.

## 📞 Soporte

Si tienes problemas:
1. Revisa los logs en la ventana de la aplicación
2. Verifica que OpenVPN funciona manualmente: `sudo openvpn --version`
3. Consulta TECHNICAL_CONTEXT.md para detalles técnicos
