# Guía de Compilación y Packaging - PreyVPN

Esta guía explica cómo compilar PreyVPN y crear el paquete .deb **sin necesidad de instalar Go ni dependencias** en tu máquina.

## 🎯 Compilación con Docker (Recomendado)

**Ventajas:**
- ✅ NO requiere instalar Go
- ✅ NO requiere instalar dependencias de Fyne (libgl, xorg-dev, libayatana-appindicator3-dev, etc.)
- ✅ Entorno reproducible
- ✅ Funciona en cualquier máquina con Docker

### Requisito Único

Solo necesitas **Docker** instalado:

```bash
# Verificar que Docker está instalado
docker --version
```

Si no tienes Docker: https://docs.docker.com/get-docker/

---

## 📦 Opción 1: Compilar con Taskfile

Si tienes [Task](https://taskfile.dev/installation/) instalado:

```bash
# Compilar binario para desarrollo
task build-docker

# O compilar versión optimizada para distribución
task build-docker-release
```

El binario estará en `./dist/preyvpn`

---

## 📦 Opción 2: Compilar con script dev.sh

```bash
# Compilar binario
./dev.sh build-binary
```

El binario estará en `./dist/preyvpn`

---

## 📦 Opción 3: Compilar con Docker directamente

```bash
# 1. Crear directorio de salida
mkdir -p dist

# 2. Construir imagen de compilación
docker build -f Dockerfile.build -t preyvpn-builder --target builder .

# 3. Compilar y extraer binario
docker run --rm -v $(pwd)/dist:/output preyvpn-builder \
    sh -c "cp /build/preyvpn /output/ && chmod +x /output/preyvpn"

# 4. Verificar el binario
ls -lh dist/preyvpn
file dist/preyvpn
```

---

## ⏱️ Tiempos de Compilación

| Acción | Primera vez | Siguientes veces |
|--------|-------------|------------------|
| Construir imagen | ~5-7 min | ~10 seg (cache) |
| Compilar binario | ~3-5 min | ~10 seg (cache) |
| **Total** | **~8-12 min** | **~20 seg** |

**Nota:** La primera vez toma más tiempo porque Docker descarga las imágenes base y compila todas las dependencias. Las siguientes compilaciones son **mucho más rápidas** gracias al cache de Docker.

---

## 🚀 Ejecutar el Binario Compilado

```bash
# Verificar que existe
ls -lh dist/preyvpn

# Ejecutar
./dist/preyvpn
```

**Requisitos para ejecutar:**
- OpenVPN instalado: `sudo apt install openvpn`
- Archivo de configuración en: `~/PreyVPN/prey-prod.ovpn`

---

## 🔧 Compilación para Múltiples Plataformas

### Linux (nativo)

```bash
# AMD64 (Intel/AMD de 64 bits)
task build-docker

# ARM64 (Raspberry Pi 4, servidores ARM)
docker build -f Dockerfile.build -t preyvpn-builder \
    --build-arg GOARCH=arm64 --target builder .
docker run --rm -v $(pwd)/dist:/output preyvpn-builder \
    sh -c "cp /build/preyvpn /output/preyvpn-arm64 && chmod +x /output/preyvpn-arm64"
```

### Windows (cross-compilation desde Linux)

```bash
# Requiere mingw-w64 en la imagen
docker build -f Dockerfile.build -t preyvpn-builder-windows \
    --build-arg GOOS=windows --build-arg GOARCH=amd64 --target builder .
docker run --rm -v $(pwd)/dist:/output preyvpn-builder-windows \
    sh -c "cp /build/preyvpn.exe /output/"
```

### macOS (cross-compilation desde Linux)

```bash
# Requiere osxcross en la imagen
docker build -f Dockerfile.build -t preyvpn-builder-darwin \
    --build-arg GOOS=darwin --build-arg GOARCH=amd64 --target builder .
docker run --rm -v $(pwd)/dist:/output preyvpn-builder-darwin \
    sh -c "cp /build/preyvpn /output/preyvpn-darwin"
```

---

## 🐛 Solución de Problemas

### Error: "Cannot connect to the Docker daemon"

```bash
# Verificar que Docker está corriendo
sudo systemctl start docker

# O en macOS/Windows
# Abrir Docker Desktop
```

### Error: "permission denied" al ejecutar el binario

```bash
chmod +x dist/preyvpn
```

### El binario no se creó

```bash
# Ver logs de compilación
docker build -f Dockerfile.build -t preyvpn-builder --target builder . 2>&1 | tee build.log
```

### Limpiar cache de Docker

Si necesitas recompilar desde cero:

```bash
# Limpiar cache de build
docker builder prune -a

# O eliminar la imagen y reconstruir
docker rmi preyvpn-builder
task build-docker
```

---

## 📊 Comparación: Docker vs Local

| Aspecto | Compilación Docker | Compilación Local |
|---------|-------------------|-------------------|
| **Instalación Go** | ❌ No requerido | ✅ Requerido |
| **Dependencias** | ❌ No requerido | ✅ Requerido |
| **Primera compilación** | ~8-12 min | ~5-7 min |
| **Siguientes compilaciones** | ~20 seg | ~10 seg |
| **Reproducibilidad** | ✅ 100% | ⚠️ Depende del entorno |
| **Tamaño del binario** | ~27 MB | ~27 MB |

---

## 💡 Tips

1. **Cache de Docker**: La primera compilación toma tiempo, pero las siguientes son rápidas gracias al cache de layers.

2. **Compilar en background**:
   ```bash
   task build-docker > build.log 2>&1 &
   tail -f build.log
   ```

3. **Verificar el binario**:
   ```bash
   # Ver información del archivo
   file dist/preyvpn

   # Ver tamaño
   ls -lh dist/preyvpn

   # Ver dependencias dinámicas
   ldd dist/preyvpn
   ```

4. **Optimizar tamaño**:
   ```bash
   # Usar build-docker-release que incluye strip
   task build-docker-release

   # Reduce el binario de ~35MB a ~27MB
   ```

---

## 🔗 Recursos

- **Dockerfile.build**: Configuración del entorno de compilación
- **Taskfile.yml**: Comandos automatizados
- **dev.sh**: Script alternativo para compilación
- **DOCKER-README.md**: Documentación del entorno de desarrollo

---

## ❓ Preguntas Frecuentes

### ¿Puedo compilar sin Docker?

Sí, pero necesitarás instalar:
- Go 1.22+
- Dependencias de Fyne: `sudo apt install libgl1-mesa-dev xorg-dev`
- OpenVPN: `sudo apt install openvpn`

Ver [README.md](README.md) para instrucciones de compilación local.

### ¿El binario funciona en cualquier distro de Linux?

El binario está compilado para Linux genérico y debería funcionar en:
- Ubuntu 20.04+
- Debian 11+
- Fedora 35+
- Arch Linux
- Otras distros con glibc 2.31+

### ¿Puedo distribuir el binario compilado?

Sí, el binario en `dist/preyvpn` es autocontenido y puede distribuirse a otros usuarios de Linux. Solo necesitan tener OpenVPN instalado.

### ¿Cómo actualizar las dependencias?

```bash
# Actualizar go.mod
go get -u ./...
go mod tidy

# Reconstruir imagen sin cache
docker build --no-cache -f Dockerfile.build -t preyvpn-builder --target builder .
```

---

## 📦 Creación del Paquete .deb

### Requisitos

- Binario compilado en `dist/preyvpn`
- Python 3 con PIL (Pillow) para generar el icono
- `dpkg-deb` (viene instalado en Ubuntu/Debian)
- Opcionalmente: `fakeroot` (recomendado)

### Paso 1: Compilar el binario

```bash
# Con Docker (recomendado)
./dev.sh build-binary
# O
task build-docker

# El binario estará en dist/preyvpn
ls -lh dist/preyvpn
```

### Paso 2: Generar el icono de la aplicación

```bash
# Instalar dependencias de Python (solo primera vez)
pip3 install Pillow

# Generar el icono
cd packaging
python3 create-icon.py

# Verificar que se creó
ls -lh debian/usr/share/icons/hicolor/256x256/apps/preyvpn.png
cd ..
```

**Nota:** El script `create-icon.py` genera un icono simple con una "V" estilizada en un círculo verde. Puedes reemplazarlo con tu propio icono PNG de 256x256.

### Paso 3: Construir el paquete .deb

```bash
cd packaging
./build-deb.sh

# El paquete se creará en dist/
ls -lh ../dist/preyvpn_1.0.0_amd64.deb
```

### Proceso completo en un solo comando

```bash
# Compilar binario
./dev.sh build-binary

# Crear icono (si no existe)
cd packaging && python3 create-icon.py && cd ..

# Construir .deb
cd packaging && ./build-deb.sh && cd ..

# ¡Listo! El paquete está en dist/preyvpn_1.0.0_amd64.deb
```

### Verificar el paquete

```bash
# Ver información del paquete
dpkg-deb --info dist/preyvpn_1.0.0_amd64.deb

# Ver contenido del paquete
dpkg-deb --contents dist/preyvpn_1.0.0_amd64.deb

# Verificar dependencias
dpkg-deb --field dist/preyvpn_1.0.0_amd64.deb Depends
```

### Probar la instalación

```bash
# Instalar el paquete
sudo dpkg -i dist/preyvpn_1.0.0_amd64.deb

# Si hay errores de dependencias
sudo apt-get install -f

# Verificar que se instaló
which preyvpn
dpkg -l | grep preyvpn

# Ejecutar desde el menú de aplicaciones
# O desde terminal:
preyvpn

# Desinstalar (si quieres)
sudo apt remove preyvpn
```

---

## 📋 Estructura del Paquete .deb

El script `packaging/build-deb.sh` crea la siguiente estructura:

```
packaging/debian/
├── DEBIAN/
│   ├── control                # Metadata del paquete y dependencias
│   ├── postinst               # Script que se ejecuta después de instalar
│   └── prerm                  # Script que se ejecuta antes de desinstalar
├── usr/
│   ├── bin/
│   │   └── preyvpn           # Binario copiado de dist/
│   └── share/
│       ├── applications/
│       │   └── preyvpn.desktop  # Entrada en el menú de aplicaciones
│       └── icons/hicolor/256x256/apps/
│           └── preyvpn.png      # Icono de la aplicación
```

### Archivos importantes

#### control
Define el paquete y sus dependencias:
```
Package: preyvpn
Version: 1.0.0
Architecture: amd64
Depends: openvpn, policykit-1, libgl1, libayatana-appindicator3-1, ...
```

#### postinst
Configura `/etc/sudoers.d/preyvpn` para que **todos los usuarios** puedan ejecutar openvpn sin password:
```bash
echo "ALL ALL=(ALL) NOPASSWD: /usr/sbin/openvpn" > /etc/sudoers.d/preyvpn
chmod 0440 /etc/sudoers.d/preyvpn
```

#### prerm
Limpia la configuración de sudo al desinstalar:
```bash
rm -f /etc/sudoers.d/preyvpn
```

---

## 🔧 Personalizar el paquete

### Cambiar la versión

Edita estos archivos:
- `packaging/debian/DEBIAN/control` - línea `Version:`
- `packaging/build-deb.sh` - variable `PACKAGE_NAME`

### Cambiar dependencias

Edita `packaging/debian/DEBIAN/control` - línea `Depends:`

### Cambiar el icono

Reemplaza `packaging/debian/usr/share/icons/hicolor/256x256/apps/preyvpn.png` con tu propio icono PNG de 256x256.

### Agregar más archivos

Agrega archivos en `packaging/debian/usr/` siguiendo la estructura de directorios de Linux.

Por ejemplo, para agregar documentación:
```bash
mkdir -p packaging/debian/usr/share/doc/preyvpn
cp README.md packaging/debian/usr/share/doc/preyvpn/
```

---

## 🚨 Troubleshooting del Packaging

### Error: "control file must have a newline at end"
Asegúrate de que `packaging/debian/DEBIAN/control` tenga una línea vacía al final.

### Error: "cannot stat 'dist/preyvpn': No such file or directory"
Compila el binario primero con `./dev.sh build-binary`

### Error: "Installed-Size appears twice"
El script `build-deb.sh` calcula automáticamente el tamaño. No modifiques el control manualmente durante el build.

### El paquete no instala las dependencias
Usa `sudo apt-get install -f` después de instalar con dpkg.

### Los permisos no funcionan después de instalar
Verifica que el script postinst se ejecutó:
```bash
cat /etc/sudoers.d/preyvpn
# Debería mostrar: ALL ALL=(ALL) NOPASSWD: /usr/sbin/openvpn
```

Si no existe, reinstala:
```bash
sudo apt remove preyvpn
sudo dpkg -i dist/preyvpn_1.0.0_amd64.deb
```

---

**¿Más preguntas?** Consulta [DOCKER-README.md](DOCKER-README.md) o [ARCHITECTURE.md](ARCHITECTURE.md) para más detalles.
