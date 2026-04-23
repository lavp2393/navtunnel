package logs

import (
	"strings"
	"sync"
)

// Buffer mantiene un buffer circular de líneas de log
type Buffer struct {
	lines    []string
	maxLines int
	mu       sync.RWMutex
}

// NewBuffer crea un nuevo buffer de logs con capacidad máxima
func NewBuffer(maxLines int) *Buffer {
	return &Buffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

// Add agrega una nueva línea al buffer
func (b *Buffer) Add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Sanitizar la línea (eliminar posibles secretos)
	sanitized := sanitizeLine(line)

	// Si llegamos al máximo, eliminar la más antigua
	if len(b.lines) >= b.maxLines {
		b.lines = b.lines[1:]
	}

	b.lines = append(b.lines, sanitized)
}

// GetAll retorna todas las líneas actuales
func (b *Buffer) GetAll() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Retornar una copia para evitar race conditions
	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// GetText retorna todas las líneas como un string único
func (b *Buffer) GetText() string {
	lines := b.GetAll()
	return strings.Join(lines, "\n")
}

// Clear limpia el buffer
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = make([]string, 0, b.maxLines)
}

// Count retorna el número de líneas actuales
func (b *Buffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.lines)
}

// sanitizeLine elimina información sensible de las líneas de log.
// Solo filtra formatos conocidos de comandos al management interface
// ("username <algo> <secreto>", "password <algo> <secreto>") para no
// destruir líneas legítimas de OpenVPN que mencionen la palabra "password".
func sanitizeLine(line string) string {
	if strings.HasPrefix(line, "username ") || strings.HasPrefix(line, "password ") {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			return parts[0] + " " + parts[1] + " ********"
		}
	}
	return line
}
