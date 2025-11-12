#!/bin/bash
# Script para construir el paquete .deb de PreyVPN

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$SCRIPT_DIR/debian"
OUTPUT_DIR="$PROJECT_ROOT/dist"

echo "🔨 Construyendo paquete .deb para PreyVPN"
echo ""

# Verificar que existe el binario compilado
if [ ! -f "$PROJECT_ROOT/dist/preyvpn" ]; then
    echo "❌ Error: No se encontró el binario compilado en dist/preyvpn"
    echo "   Por favor compila primero con: ./dev.sh build-binary"
    exit 1
fi

# Crear directorio de salida si no existe
mkdir -p "$OUTPUT_DIR"

# Copiar el binario al directorio del paquete
echo "📦 Copiando binario..."
cp "$PROJECT_ROOT/dist/preyvpn" "$BUILD_DIR/usr/bin/preyvpn"
chmod 755 "$BUILD_DIR/usr/bin/preyvpn"

# Verificar estructura
echo "📋 Verificando estructura del paquete..."
if [ ! -f "$BUILD_DIR/DEBIAN/control" ]; then
    echo "❌ Error: Falta archivo DEBIAN/control"
    exit 1
fi

if [ ! -f "$BUILD_DIR/DEBIAN/postinst" ]; then
    echo "❌ Error: Falta archivo DEBIAN/postinst"
    exit 1
fi

# Asegurar permisos correctos
echo "🔐 Configurando permisos..."
chmod 755 "$BUILD_DIR/DEBIAN/postinst"
chmod 755 "$BUILD_DIR/DEBIAN/prerm"
chmod 644 "$BUILD_DIR/DEBIAN/control"
chmod 644 "$BUILD_DIR/usr/share/applications/preyvpn.desktop"
chmod 644 "$BUILD_DIR/usr/share/icons/hicolor/256x256/apps/preyvpn.png"

# Calcular tamaño instalado (en KB)
INSTALLED_SIZE=$(du -sk "$BUILD_DIR" | cut -f1)
echo "Installed-Size: $INSTALLED_SIZE" >> "$BUILD_DIR/DEBIAN/control.tmp"
cat "$BUILD_DIR/DEBIAN/control" >> "$BUILD_DIR/DEBIAN/control.tmp"
mv "$BUILD_DIR/DEBIAN/control.tmp" "$BUILD_DIR/DEBIAN/control"

# Construir el paquete
echo "🏗️  Construyendo paquete .deb..."
PACKAGE_NAME="preyvpn_1.0.0_amd64.deb"

# Usar fakeroot si está disponible (mejor práctica)
if command -v fakeroot &> /dev/null; then
    fakeroot dpkg-deb --build "$BUILD_DIR" "$OUTPUT_DIR/$PACKAGE_NAME"
else
    dpkg-deb --build "$BUILD_DIR" "$OUTPUT_DIR/$PACKAGE_NAME"
fi

# Verificar que se creó el paquete
if [ -f "$OUTPUT_DIR/$PACKAGE_NAME" ]; then
    echo ""
    echo "✅ Paquete .deb creado exitosamente!"
    echo ""
    echo "📦 Archivo: $OUTPUT_DIR/$PACKAGE_NAME"
    ls -lh "$OUTPUT_DIR/$PACKAGE_NAME"
    echo ""
    echo "📋 Información del paquete:"
    dpkg-deb --info "$OUTPUT_DIR/$PACKAGE_NAME"
    echo ""
    echo "📂 Contenido del paquete:"
    dpkg-deb --contents "$OUTPUT_DIR/$PACKAGE_NAME"
    echo ""
    echo "🚀 Para instalar:"
    echo "   sudo dpkg -i $OUTPUT_DIR/$PACKAGE_NAME"
    echo ""
    echo "   O con doble clic en el archivo desde el explorador de archivos"
    echo ""
else
    echo "❌ Error: No se pudo crear el paquete .deb"
    exit 1
fi
