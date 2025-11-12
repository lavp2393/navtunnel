#!/usr/bin/env python3
"""
Genera iconos PNG simples para el system tray
Requiere: pip install pillow
"""

from PIL import Image, ImageDraw

def create_circle_icon(filename, color, size=32):
    """Crea un icono circular simple"""
    # Crear imagen con transparencia
    img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Dibujar círculo con borde
    margin = 4
    draw.ellipse(
        [margin, margin, size-margin, size-margin],
        fill=color,
        outline=(50, 50, 50, 255),
        width=2
    )

    img.save(filename, 'PNG')
    print(f"✓ Created {filename}")

if __name__ == '__main__':
    # Gris para desconectado
    create_circle_icon('disconnected.png', (128, 128, 128, 255))

    # Amarillo/naranja para conectando
    create_circle_icon('connecting.png', (255, 165, 0, 255))

    # Verde para conectado
    create_circle_icon('connected.png', (0, 200, 0, 255))

    # Rojo para error
    create_circle_icon('error.png', (220, 20, 20, 255))

    print("\n✅ All icons generated successfully!")
