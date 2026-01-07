package models

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

var (
	ErrNoPortsAvailable = errors.New("no ports available")
	ErrPortInUse        = errors.New("port is in use")
)

// PortAllocator manages dynamic port allocation for models
type PortAllocator struct {
	startPort int
	endPort   int
	allocated map[int]string // port -> model name
	mu        sync.Mutex
}

// NewPortAllocator creates a new port allocator
func NewPortAllocator(startPort int) *PortAllocator {
	return &PortAllocator{
		startPort: startPort,
		endPort:   startPort + 1000, // Allow 1000 ports
		allocated: make(map[int]string),
	}
}

// Allocate allocates a port for the given model
func (pa *PortAllocator) Allocate(modelName string) (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Check if model already has a port
	for port, name := range pa.allocated {
		if name == modelName {
			return port, nil
		}
	}

	// Find an available port
	for port := pa.startPort; port < pa.endPort; port++ {
		if _, exists := pa.allocated[port]; exists {
			continue
		}

		// Check if port is actually available on the system
		if !isPortAvailable(port) {
			continue
		}

		pa.allocated[port] = modelName
		return port, nil
	}

	return 0, ErrNoPortsAvailable
}

// Release releases a port for a model
func (pa *PortAllocator) Release(modelName string) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	for port, name := range pa.allocated {
		if name == modelName {
			delete(pa.allocated, port)
			return
		}
	}
}

// GetPort returns the port allocated to a model, or 0 if none
func (pa *PortAllocator) GetPort(modelName string) int {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	for port, name := range pa.allocated {
		if name == modelName {
			return port
		}
	}
	return 0
}

// IsAllocated checks if a port is allocated
func (pa *PortAllocator) IsAllocated(port int) bool {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	_, exists := pa.allocated[port]
	return exists
}

// AllocatedPorts returns a copy of all allocated ports
func (pa *PortAllocator) AllocatedPorts() map[int]string {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	result := make(map[int]string)
	for port, name := range pa.allocated {
		result[port] = name
	}
	return result
}

// isPortAvailable checks if a port is available on the system
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
