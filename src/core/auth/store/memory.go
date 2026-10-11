package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryAuthStore in-memory authentication store implementation
type MemoryAuthStore struct {
	clients  map[string]*ClientInfo
	mutex    sync.RWMutex
	expiryHr int // Expiry time (hours)
}

// NewMemoryAuthStore creates in-memory authentication store
func NewMemoryAuthStore(expiryHr int) *MemoryAuthStore {
	if expiryHr <= 0 {
		expiryHr = 24 // Default 24 hours
	}

	store := &MemoryAuthStore{
		clients:  make(map[string]*ClientInfo),
		expiryHr: expiryHr,
	}

	// Start background goroutine to clean up expired data
	go store.periodicCleanup()

	return store
}

// StoreAuth stores client authentication info
func (m *MemoryAuthStore) StoreAuth(
	clientID, username, password string,
	metadata map[string]interface{},
) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if clientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}

	// Parse IP info from username (base64 encoded JSON)
	ip := ""
	deviceID := ""

	if username != "" {
		// Attempt base64 decode
		if decoded, err := base64.StdEncoding.DecodeString(username); err == nil {
			var userInfo map[string]interface{}
			if json.Unmarshal(decoded, &userInfo) == nil {
				if ipValue, ok := userInfo["ip"].(string); ok {
					ip = ipValue
				}
			}
		}
	}

	// Extract device_id from client_id (format: CGID_test@@@device_id@@@uuid)
	if parts := strings.Split(clientID, "@@@"); len(parts) >= 2 {
		deviceID = parts[1]
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(m.expiryHr) * time.Hour)

	clientInfo := &ClientInfo{
		ClientID:  clientID,
		Username:  username,
		Password:  password,
		IP:        ip,
		DeviceID:  deviceID,
		CreatedAt: now,
		ExpiresAt: &expiresAt,
		Metadata:  metadata,
	}

	m.clients[clientID] = clientInfo

	return nil
}

// ValidateAuth validates client authentication info
func (m *MemoryAuthStore) ValidateAuth(
	clientID, username, password string,
) (bool, *ClientInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	clientInfo, exists := m.clients[clientID]
	if !exists {
		return false, nil, nil
	}

	// Check if expired
	if clientInfo.ExpiresAt != nil && time.Now().After(*clientInfo.ExpiresAt) {
		return false, nil, fmt.Errorf("authentication info expired")
	}

	// Verify username and password
	if clientInfo.Username != username || clientInfo.Password != password {
		return false, nil, nil
	}

	return true, clientInfo, nil
}

// GetClientInfo retrieves client info
func (m *MemoryAuthStore) GetClientInfo(clientID string) (*ClientInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	clientInfo, exists := m.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client does not exist: %s", clientID)
	}

	// Check if expired
	if clientInfo.ExpiresAt != nil && time.Now().After(*clientInfo.ExpiresAt) {
		return nil, fmt.Errorf("client authentication expired: %s", clientID)
	}

	// Return copy to prevent external mutation
	info := *clientInfo
	return &info, nil
}

// RemoveAuth deletes client authentication info
func (m *MemoryAuthStore) RemoveAuth(clientID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.clients, clientID)
	return nil
}

// ListClients lists all client IDs
func (m *MemoryAuthStore) ListClients() ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var clients []string
	now := time.Now()

	for clientID, clientInfo := range m.clients {
		// Return only unexpired clients
		if clientInfo.ExpiresAt == nil || now.Before(*clientInfo.ExpiresAt) {
			clients = append(clients, clientID)
		}
	}

	return clients, nil
}

// CleanupExpired cleans up expired authentications
func (m *MemoryAuthStore) CleanupExpired() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	var expiredClients []string

	for clientID, clientInfo := range m.clients {
		if clientInfo.ExpiresAt != nil && now.After(*clientInfo.ExpiresAt) {
			expiredClients = append(expiredClients, clientID)
		}
	}

	for _, clientID := range expiredClients {
		delete(m.clients, clientID)
	}

	return nil
}

// Close closes storage connection
func (m *MemoryAuthStore) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear memory data
	m.clients = make(map[string]*ClientInfo)
	return nil
}

// periodicCleanup periodically cleans up expired data
func (m *MemoryAuthStore) periodicCleanup() {
	ticker := time.NewTicker(1 * time.Hour) // Clean every hour
	defer ticker.Stop()

	for range ticker.C {
		m.CleanupExpired()
	}
}

// GetStats gets storage statistics
func (m *MemoryAuthStore) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	now := time.Now()
	total := len(m.clients)
	active := 0
	expired := 0

	for _, clientInfo := range m.clients {
		if clientInfo.ExpiresAt == nil || now.Before(*clientInfo.ExpiresAt) {
			active++
		} else {
			expired++
		}
	}

	return map[string]interface{}{
		"type":    "memory",
		"total":   total,
		"active":  active,
		"expired": expired,
	}
}
