package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
	"xiaozhi-server-go/src/core/utils"
)

// SessionKeys session key struct
type SessionKeys struct {
	Key       string    `json:"key"`        // AES key (hex)
	Nonce     string    `json:"nonce"`      // AES nonce (hex)
	SessionID string    `json:"session_id"` // Session ID
	CreatedAt time.Time `json:"created_at"` // Created at
	ExpiresAt time.Time `json:"expires_at"` // Expires at
}

// CryptoManager encryption manager interface
type CryptoManager interface {
	GenerateSessionKeys(sessionID string) (*SessionKeys, error)
	GetSessionKeys(sessionID string) (*SessionKeys, error)
	RevokeSessionKeys(sessionID string) error
	CleanupExpiredKeys() error
	GetKeyStats() map[string]interface{}
}

// MemoryCryptoManager in-memory encryption manager implementation
type MemoryCryptoManager struct {
	keys   map[string]*SessionKeys
	mutex  sync.RWMutex
	logger *utils.Logger
	keyTTL time.Duration // Key TTL
}

// NewCryptoManager creates encryption manager
func NewCryptoManager(logger *utils.Logger, keyTTL time.Duration) CryptoManager {
	if keyTTL <= 0 {
		keyTTL = 24 * time.Hour // Default 24h validity
	}

	return &MemoryCryptoManager{
		keys:   make(map[string]*SessionKeys),
		logger: logger,
		keyTTL: keyTTL,
	}
}

// generateAESKey generates 128-bit AES key
func generateAESKey() (string, error) {
	key := make([]byte, 16) // 128-bit key
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate AES key: %v", err)
	}
	return hex.EncodeToString(key), nil
}

// generateNonce generates 128-bit AES nonce
func generateNonce() (string, error) {
	nonce := make([]byte, 16) // 128-bit nonce matching ESP32 16-byte expectation
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate AES nonce: %v", err)
	}
	return hex.EncodeToString(nonce), nil
}

// GenerateSessionKeys generates session keys
func (cm *MemoryCryptoManager) GenerateSessionKeys(sessionID string) (*SessionKeys, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Generate key and nonce
	key, err := generateAESKey()
	if err != nil {
		return nil, err
	}

	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	sessionKeys := &SessionKeys{
		Key:       key,
		Nonce:     nonce,
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(cm.keyTTL),
	}

	cm.mutex.Lock()
	cm.keys[sessionID] = sessionKeys
	cm.mutex.Unlock()

	cm.logger.Debug("generated session keys", map[string]interface{}{
		"session_id": sessionID,
		"expires_at": sessionKeys.ExpiresAt,
	})

	return sessionKeys, nil
}

// GetSessionKeys retrieves session keys
func (cm *MemoryCryptoManager) GetSessionKeys(sessionID string) (*SessionKeys, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	keys, exists := cm.keys[sessionID]
	if !exists {
		return nil, fmt.Errorf("session keys not found: %s", sessionID)
	}

	// Check if expired
	if time.Now().After(keys.ExpiresAt) {
		cm.mutex.RUnlock()
		cm.mutex.Lock()
		delete(cm.keys, sessionID)
		cm.mutex.Unlock()
		cm.mutex.RLock()
		return nil, fmt.Errorf("session keys expired: %s", sessionID)
	}

	return keys, nil
}

// RevokeSessionKeys revokes session keys
func (cm *MemoryCryptoManager) RevokeSessionKeys(sessionID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.keys[sessionID]; !exists {
		return fmt.Errorf("session keys not found: %s", sessionID)
	}

	delete(cm.keys, sessionID)
	cm.logger.Info("revoked session keys", map[string]interface{}{
		"session_id": sessionID,
	})

	return nil
}

// CleanupExpiredKeys cleans up expired keys
func (cm *MemoryCryptoManager) CleanupExpiredKeys() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	for sessionID, keys := range cm.keys {
		if now.After(keys.ExpiresAt) {
			delete(cm.keys, sessionID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		cm.logger.Info("cleaned up expired keys", map[string]interface{}{
			"expired_count": expiredCount,
		})
	}

	return nil
}

// GetKeyStats gets key statistics
func (cm *MemoryCryptoManager) GetKeyStats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	now := time.Now()
	totalKeys := len(cm.keys)
	expiredKeys := 0

	for _, keys := range cm.keys {
		if now.After(keys.ExpiresAt) {
			expiredKeys++
		}
	}

	return map[string]interface{}{
		"total_keys":   totalKeys,
		"active_keys":  totalKeys - expiredKeys,
		"expired_keys": expiredKeys,
		"key_ttl":      cm.keyTTL.String(),
	}
}
