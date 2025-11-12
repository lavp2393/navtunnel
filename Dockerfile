# Dockerfile para desarrollo de PreyVPN
FROM golang:1.22-bookworm

# Instalar dependencias del sistema
RUN apt-get update && apt-get install -y \
    # Dependencias de Fyne
    libgl1-mesa-dev \
    xorg-dev \
    libx11-dev \
    libxrandr-dev \
    libxcursor-dev \
    libxinerama-dev \
    libxi-dev \
    libxxf86vm-dev \
    # Dependencias de systray
    pkg-config \
    libayatana-appindicator3-dev \
    # OpenVPN para pruebas
    openvpn \
    # Herramientas útiles
    sudo \
    git \
    curl \
    vim \
    htop \
    net-tools \
    iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Instalar Air para hot-reload (versión compatible con Go 1.22)
RUN go install github.com/air-verse/air@v1.52.3

# Configurar workspace
WORKDIR /app

# Copiar go.mod y go.sum primero (para cachear dependencias)
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código
COPY . .

# Configurar usuario para evitar problemas de permisos
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd -g ${GROUP_ID} developer || true && \
    useradd -m -u ${USER_ID} -g ${GROUP_ID} -s /bin/bash developer && \
    echo "developer ALL=(ALL) NOPASSWD: /usr/sbin/openvpn" > /etc/sudoers.d/developer && \
    chmod 0440 /etc/sudoers.d/developer && \
    chown -R developer:developer /app

USER developer

# Puerto para posibles servicios (no necesario para GUI pero útil)
EXPOSE 8080

# Comando por defecto: iniciar Air para hot-reload
CMD ["air", "-c", ".air.toml"]
