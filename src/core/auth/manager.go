package auth

import (
	"fmt"
	"sync"
	"time"
	"xiaozhi-server-go/src/core/auth/store"
	"xiaozhi-server-go/src/core/utils"
)

// AuthManager authentication manager
type AuthManager struct {
	store         store.AuthStore
	cryptoManager CryptoManager
	logger        *utils.Logger
	mutex         sync.RWMutex
}

// NewAuthManager creates an authentication manager
func NewAuthManager(storeConfig *store.StoreConfig, logger *utils.Logger) (*AuthManager, error) {
	if storeConfig == nil {
		storeConfig = store.GetDefaultStoreConfig()
	}

	// Validate configuration
	if err := store.ValidateStoreConfig(storeConfig); err != nil {
		return nil, fmt.Errorf("auth store config validation failed: %v", err)
	}

	// Create store instance
	authStore, err := store.CreateAuthStore(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth store: %v", err)
	}

	// Create encryption manager
	cryptoManager := NewCryptoManager(logger, 24*time.Hour) // 24-hour key validity

	manager := &AuthManager{
		store:         authStore,
		cryptoManager: cryptoManager,
		logger:        logger,
	}

	manager.logger.Info("[Auth] [Manager] initialized successfully", map[string]interface{}{
		"store_type": storeConfig.Type,
		"expiry_hr":  storeConfig.ExpiryHr,
	})

	return manager, nil
}

// RegisterClient registers client authentication info
func (am *AuthManager) RegisterClient(
	clientID, username, password string,
	metadata map[string]interface{},
) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if clientID == "" {
		return fmt.Errorf("client ID cannot be empty")
	}

	// Store authentication info
	err := am.store.StoreAuth(clientID, username, password, metadata)
	if err != nil {
		am.logger.Error("failed to register client auth info", map[string]interface{}{
			"client_id": clientID,
			"error":     err.Error(),
		})
		return err
	}
	return nil
}

// AuthenticateClient verifies client authentication
func (am *AuthManager) AuthenticateClient(
	clientID, username, password string,
) (bool, *store.ClientInfo, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	valid, clientInfo, err := am.store.ValidateAuth(clientID, username, password)
	if err != nil {
		am.logger.Error("client authentication verification failed", map[string]interface{}{
			"client_id": clientID,
			"error":     err.Error(),
		})
		return false, nil, err
	}

	if !valid {
		am.logger.Debug("client authentication failed", map[string]interface{}{
			"client_id": clientID,
			"username":  username,
		})
		return false, nil, nil
	}

	am.logger.Debug("client authentication successful", map[string]interface{}{
		"client_id": clientID,
		"username":  username,
		"ip":        clientInfo.IP,
		"device_id": clientInfo.DeviceID,
	})

	return true, clientInfo, nil
}

// GetClientInfo gets client info
func (am *AuthManager) GetClientInfo(clientID string) (*store.ClientInfo, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	return am.store.GetClientInfo(clientID)
}

// RemoveClient removes client authentication info
func (am *AuthManager) RemoveClient(clientID string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	err := am.store.RemoveAuth(clientID)
	if err != nil {
		am.logger.Error("failed to remove client auth info", map[string]interface{}{
			"client_id": clientID,
			"error":     err.Error(),
		})
		return err
	}

	am.logger.Info("client auth info removed", map[string]interface{}{
		"client_id": clientID,
	})

	return nil
}

// ListClients lists all authenticated clients
func (am *AuthManager) ListClients() ([]string, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	return am.store.ListClients()
}

// CleanupExpired cleans up expired authentication info
func (am *AuthManager) CleanupExpired() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	err := am.store.CleanupExpired()
	if err != nil {
		am.logger.Error("failed to cleanup expired auth info", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	am.logger.Info("expired auth info cleanup completed")
	return nil
}

// GetStats gets auth store statistics
func (am *AuthManager) GetStats() map[string]interface{} {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// Check if store has stats method
	if memStore, ok := am.store.(*store.MemoryAuthStore); ok {
		return memStore.GetStats()
	}

	// Return basic info for other store types
	clients, _ := am.store.ListClients()
	return map[string]interface{}{
		"type":   "unknown",
		"active": len(clients),
	}
}

// Close closes authentication manager
func (am *AuthManager) Close() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.store != nil {
		err := am.store.Close()
		if err != nil {
			am.logger.Error("failed to close auth store", map[string]interface{}{
				"error": err.Error(),
			})
			return err
		}
	}

	am.logger.Info("auth manager closed")
	return nil
}

// Encryption methods

// GenerateSessionKeys generates session keys
func (am *AuthManager) GenerateSessionKeys(sessionID string) (*SessionKeys, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	return am.cryptoManager.GenerateSessionKeys(sessionID)
}

// GetSessionKeys gets session keys
func (am *AuthManager) GetSessionKeys(sessionID string) (*SessionKeys, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	return am.cryptoManager.GetSessionKeys(sessionID)
}

// RevokeSessionKeys revokes session keys
func (am *AuthManager) RevokeSessionKeys(sessionID string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	return am.cryptoManager.RevokeSessionKeys(sessionID)
}

// CleanupExpiredKeys cleans up expired keys
func (am *AuthManager) CleanupExpiredKeys() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	return am.cryptoManager.CleanupExpiredKeys()
}

// GetCryptoStats gets crypto statistics
func (am *AuthManager) GetCryptoStats() map[string]interface{} {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	return am.cryptoManager.GetKeyStats()
}
