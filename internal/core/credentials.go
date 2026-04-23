package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	// serviceName identifica a la aplicación dentro del keyring
	serviceName = "NavTunnel"
	// keyringUser es el identificador único para las credenciales almacenadas
	keyringUser = "credentials"
)

var (
	// ErrCredentialsNotFound se usa cuando no existen credenciales guardadas
	ErrCredentialsNotFound = errors.New("credentials not found")
)

// SavedCredentials representa la estructura guardada en el keyring o en disco
type SavedCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CredentialStoreMethod indica dónde se guardaron las credenciales
type CredentialStoreMethod string

const (
	CredentialStoreMethodNone    CredentialStoreMethod = ""
	CredentialStoreMethodKeyring CredentialStoreMethod = "keyring"
	CredentialStoreMethodFile    CredentialStoreMethod = "file"
)

// SaveCredentials guarda las credenciales. Devuelve el método utilizado y un warning opcional.
func SaveCredentials(username, password string) (CredentialStoreMethod, string, error) {
	creds := SavedCredentials{
		Username: username,
		Password: password,
	}

	if keyringErr := saveToKeyring(creds); keyringErr == nil {
		// Si el keyring funciona eliminamos cualquier fallback previo
		_ = deleteFallbackFile()
		return CredentialStoreMethodKeyring, "", nil
	} else {
		warn := keyringWarning(keyringErr)
		if fallbackErr := saveToFile(creds); fallbackErr != nil {
			if warn != "" {
				return CredentialStoreMethodNone, "", fmt.Errorf("keyring unavailable (%v) and fallback failed: %w", keyringErr, fallbackErr)
			}
			return CredentialStoreMethodNone, "", fmt.Errorf("failed to store credentials: %w", fallbackErr)
		}
		return CredentialStoreMethodFile, warn, nil
	}
}

// LoadCredentials intenta recuperar las credenciales almacenadas.
// Retorna username, password, método empleado, warning opcional y error.
func LoadCredentials() (string, string, CredentialStoreMethod, string, error) {
	if creds, err := loadFromKeyring(); err == nil {
		return creds.Username, creds.Password, CredentialStoreMethodKeyring, "", nil
	} else {
		warning := ""
		if !errors.Is(err, keyring.ErrNotFound) {
			warning = keyringWarning(err)
		}

		if fallbackCreds, ferr := loadFromFile(); ferr == nil {
			return fallbackCreds.Username, fallbackCreds.Password, CredentialStoreMethodFile, warning, nil
		} else if errors.Is(ferr, ErrCredentialsNotFound) {
			if errors.Is(err, keyring.ErrNotFound) {
				return "", "", CredentialStoreMethodNone, "", ErrCredentialsNotFound
			}
			if warning != "" {
				return "", "", CredentialStoreMethodNone, warning, ErrCredentialsNotFound
			}
			return "", "", CredentialStoreMethodNone, "", ErrCredentialsNotFound
		} else {
			if warning != "" {
				return "", "", CredentialStoreMethodNone, warning, ferr
			}
			return "", "", CredentialStoreMethodNone, "", ferr
		}
	}
}

// DeleteCredentials elimina las credenciales almacenadas (keyring y fallback)
func DeleteCredentials() error {
	var resultErr error

	if err := keyring.Delete(serviceName, keyringUser); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		resultErr = err
	}

	if err := deleteFallbackFile(); err != nil {
		if resultErr == nil {
			resultErr = err
		} else {
			resultErr = fmt.Errorf("%v; fallback delete failed: %w", resultErr, err)
		}
	}

	return resultErr
}

// HasCredentials indica si existen credenciales guardadas
func HasCredentials() bool {
	_, _, _, _, err := LoadCredentials()
	return err == nil
}

func saveToKeyring(creds SavedCredentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return keyring.Set(serviceName, keyringUser, string(data))
}

func loadFromKeyring() (SavedCredentials, error) {
	data, err := keyring.Get(serviceName, keyringUser)
	if err != nil {
		return SavedCredentials{}, err
	}

	var creds SavedCredentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return SavedCredentials{}, err
	}

	if creds.Username == "" || creds.Password == "" {
		return SavedCredentials{}, ErrCredentialsNotFound
	}

	return creds, nil
}

func saveToFile(creds SavedCredentials) error {
	path, err := fallbackPath(true)
	if err != nil {
		return err
	}

	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	return os.Chmod(path, 0o600)
}

func loadFromFile() (SavedCredentials, error) {
	path, err := fallbackPath(false)
	if err != nil {
		return SavedCredentials{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SavedCredentials{}, ErrCredentialsNotFound
		}
		return SavedCredentials{}, err
	}

	if len(data) == 0 {
		return SavedCredentials{}, ErrCredentialsNotFound
	}

	var creds SavedCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return SavedCredentials{}, err
	}

	if creds.Username == "" || creds.Password == "" {
		return SavedCredentials{}, ErrCredentialsNotFound
	}

	return creds, nil
}

func deleteFallbackFile() error {
	path, err := fallbackPath(false)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func fallbackPath(createDir bool) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			if err != nil {
				return "", fmt.Errorf("cannot determine config dir: %v / %v", err, herr)
			}
			return "", herr
		}
		configDir = filepath.Join(home, ".navtunnel")
	} else {
		configDir = filepath.Join(configDir, "NavTunnel")
	}

	if createDir {
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return "", err
		}
	}

	return filepath.Join(configDir, "credentials.json"), nil
}

// GetCredentialsFallbackPath devuelve la ruta del archivo de fallback (si se puede determinar)
func GetCredentialsFallbackPath() string {
	path, err := fallbackPath(false)
	if err != nil {
		return ""
	}
	return path
}

func keyringWarning(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("no se pudo acceder al keyring del sistema: %v", err)
}
