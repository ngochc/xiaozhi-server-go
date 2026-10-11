package transport

import (
	"context"
	"sync"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"
)

// TransportManager transport manager
type TransportManager struct {
	transports map[string]Transport
	logger     *utils.Logger
	config     *configs.Config
	mu         sync.RWMutex
}

// NewTransportManager creates a new transport manager
func NewTransportManager(config *configs.Config, logger *utils.Logger) *TransportManager {
	return &TransportManager{
		transports: make(map[string]Transport),
		logger:     logger,
		config:     config,
	}
}

// RegisterTransport registers a transport layer
func (m *TransportManager) RegisterTransport(name string, transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transports[name] = transport
	m.logger.Debug("registered transport layer: %s (%s)", name, transport.GetType())
}

// StartAll starts all transport layers
func (m *TransportManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, transport := range m.transports {
		// m.logger.Info(fmt.Sprintf("starting transport layer: %s", name))

		// Launch separate goroutine for each transport layer
		go func(name string, transport Transport) {
			if err := transport.Start(ctx); err != nil {
				m.logger.Error("transport layer %s failed: %v", name, err)
			}
		}(name, transport)
	}

	m.StartTicker(ctx)

	return nil
}

func (m *TransportManager) StartTicker(ctx context.Context) {
	// Set ticker to print status for each transport layer
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				clientCnt := 0
				sessionCnt := 0
				for _, transport := range m.transports {
					c, s := transport.GetActiveConnectionCount()
					clientCnt += c
					sessionCnt += s
				}
				//m.logger.Info("Active connections: %d, Active sessions: %d", clientCnt, sessionCnt)
				systemMemoryUse, _ := utils.GetSystemMemoryUsage()
				systemCPUUse, _ := utils.GetSystemCPUUsage()
				database.UpdateServerStatus(systemMemoryUse, systemCPUUse, clientCnt, sessionCnt)
			}
		}
	}()
}

// StopAll stops all transport layers
func (m *TransportManager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for name, transport := range m.transports {
		if err := transport.Stop(); err != nil {
			m.logger.Error("failed to stop transport layer %s: %v", name, err)
			lastErr = err
		}
	}
	return lastErr
}

// GetTransport gets transport layer by name
func (m *TransportManager) GetTransport(name string) Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transports[name]
}
