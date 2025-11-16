package internal

import (
	"log"
	"sync"
)

// CleanupManager tracks resources and ensures ordered cleanup in LIFO order.
type CleanupManager struct {
	mu    sync.Mutex
	funcs []cleanupFunc
}

type cleanupFunc struct {
	name string
	fn   func() error
}

// NewCleanupManager creates a new cleanup manager.
func NewCleanupManager() *CleanupManager {
	return &CleanupManager{}
}

// Add registers a cleanup function. Functions are executed in LIFO order
// (last added, first executed) to ensure proper cleanup sequencing.
func (m *CleanupManager) Add(name string, fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.funcs = append([]cleanupFunc{{name, fn}}, m.funcs...)
}

// Execute runs all cleanup functions in reverse order (LIFO), logging any errors.
// This method always completes all cleanup operations, even if some fail.
func (m *CleanupManager) Execute() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cleanup := range m.funcs {
		if err := cleanup.fn(); err != nil {
			log.Printf("cleanup failed for %s: %v", cleanup.name, err)
		}
	}
}
