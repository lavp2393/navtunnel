package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config representa la configuración de la aplicación
type Config struct {
	// VPNConfigPath es la ruta al archivo .ovpn seleccionado por el usuario
	VPNConfigPath string `json:"vpn_config_path"`

	// Versión de la configuración (para futuras migraciones)
	Version int `json:"version"`
}

var (
	// ErrConfigNotFound se usa cuando no existe configuración guardada
	ErrConfigNotFound = errors.New("configuration not found")
)

// getConfigPath retorna la ruta del archivo de configuración
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Usar XDG Base Directory o ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(homeDir, ".config")
	}

	configDir := filepath.Join(configHome, "PreyVPN")

	// Crear directorio si no existe
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.json"), nil
}

// Load carga la configuración desde disco
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Save guarda la configuración a disco
func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Asegurar que la versión está configurada
	if c.Version == 0 {
		c.Version = 1
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// HasVPNConfig verifica si hay un archivo .ovpn configurado
func (c *Config) HasVPNConfig() bool {
	return c.VPNConfigPath != ""
}

// IsVPNConfigValid verifica si el archivo .ovpn configurado existe
func (c *Config) IsVPNConfigValid() bool {
	if !c.HasVPNConfig() {
		return false
	}

	_, err := os.Stat(c.VPNConfigPath)
	return err == nil
}

// GetConfigDir retorna el directorio de configuración
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configHome, "PreyVPN"), nil
}

// Default retorna una configuración por defecto
func Default() *Config {
	return &Config{
		Version: 1,
	}
}
