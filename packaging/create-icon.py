#!/usr/bin/env python3
"""
Genera un icono de 256x256 para PreyVPN
"""

from PIL import Image, ImageDraw, ImageFont

def create_app_icon(filename, size=256):
    """Crea un icono profesional para la aplicación"""
    # Crear imagen con transparencia
    img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Fondo circular con gradiente (simular usando círculo sólido)
    margin = size // 8
    circle_size = size - (margin * 2)

    # Círculo exterior (borde)
    draw.ellipse(
        [margin, margin, size-margin, size-margin],
        fill=(0, 150, 0, 255),  # Verde corporativo
        outline=(0, 100, 0, 255),
        width=4
    )

    # Círculo interior (más claro)
    inner_margin = margin + 20
    draw.ellipse(
        [inner_margin, inner_margin, size-inner_margin, size-inner_margin],
        fill=(0, 200, 0, 255),
        outline=None
    )

    # Símbolo de VPN (candado + señal)
    center = size // 2

    # Dibujar "V" estilizada en blanco
    v_size = size // 3
    v_thickness = size // 20

    # Parte izquierda de la V
    draw.line(
        [(center - v_size//2, center - v_size//3), (center, center + v_size//3)],
        fill=(255, 255, 255, 255),
        width=v_thickness
    )

    # Parte derecha de la V
    draw.line(
        [(center, center + v_size//3), (center + v_size//2, center - v_size//3)],
        fill=(255, 255, 255, 255),
        width=v_thickness
    )

    img.save(filename, 'PNG')
    print(f"✓ Created {filename} ({size}x{size})")

if __name__ == '__main__':
    create_app_icon('debian/usr/share/icons/hicolor/256x256/apps/preyvpn.png', 256)
    print("\n✅ Icon generated successfully!")
