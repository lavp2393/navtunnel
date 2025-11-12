#!/bin/bash
# Script para configurar sudo sin contraseña para OpenVPN
# Esto permite que PreyVPN funcione sin sudo y sin pkexec

set -e

echo "🔐 Configurando sudo para PreyVPN"
echo ""
echo "Este script configurará sudo para que NO pida contraseña"
echo "cuando PreyVPN ejecute OpenVPN."
echo ""
echo "⚠️  IMPORTANTE: Esto es seguro porque:"
echo "   - Solo permite ejecutar /usr/sbin/openvpn (nada más)"
echo "   - Solo para tu usuario ($USER)"
echo "   - No da acceso root general"
echo ""
read -p "¿Continuar? (s/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Ss]$ ]]; then
    echo "❌ Cancelado"
    exit 1
fi

# Buscar OpenVPN
OPENVPN_PATH=""
for path in /usr/sbin/openvpn /usr/bin/openvpn /usr/local/sbin/openvpn; do
    if [ -f "$path" ]; then
        OPENVPN_PATH="$path"
        break
    fi
done

if [ -z "$OPENVPN_PATH" ]; then
    echo "❌ OpenVPN no encontrado. Instálalo con:"
    echo "   sudo apt install openvpn"
    exit 1
fi

echo ""
echo "📍 OpenVPN encontrado en: $OPENVPN_PATH"
echo "👤 Usuario: $USER"
echo ""

# Crear archivo sudoers
SUDOERS_FILE="/etc/sudoers.d/preyvpn-openvpn"
SUDOERS_CONTENT="# PreyVPN - Permitir ejecutar OpenVPN sin contraseña
$USER ALL=(ALL) NOPASSWD: $OPENVPN_PATH"

echo "📝 Creando configuración en: $SUDOERS_FILE"
echo "$SUDOERS_CONTENT" | sudo tee "$SUDOERS_FILE" > /dev/null

# Configurar permisos correctos (CRÍTICO para sudoers)
sudo chmod 0440 "$SUDOERS_FILE"

# Validar sintaxis
if sudo visudo -c -f "$SUDOERS_FILE" > /dev/null 2>&1; then
    echo "✅ Configuración creada correctamente"
    echo ""
    echo "✨ Ahora puedes ejecutar PreyVPN sin sudo:"
    echo "   ./dist/preyvpn"
    echo ""
    echo "🔒 Para deshacer esta configuración, ejecuta:"
    echo "   sudo rm $SUDOERS_FILE"
else
    echo "❌ Error en la configuración. Eliminando..."
    sudo rm -f "$SUDOERS_FILE"
    exit 1
fi
