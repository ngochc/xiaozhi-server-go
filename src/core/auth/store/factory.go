package store

import (
	"fmt"
)

// CreateAuthStore creates an auth store instance based on config
func CreateAuthStore(config *StoreConfig) (AuthStore, error) {
	if config == nil {
		return nil, fmt.Errorf("store config cannot be empty")
	}

	switch config.Type {
	case "database", "":
		// Default to database store
		expiryHr := config.ExpiryHr
		if expiryHr <= 0 {
			expiryHr = 24 // Default 24 hours
		}
		return NewDatabaseAuthStore(expiryHr), nil

	case "memory":
		// Optional: in-memory store
		expiryHr := config.ExpiryHr
		if expiryHr <= 0 {
			expiryHr = 24 // Default 24 hours
		}
		return NewMemoryAuthStore(expiryHr), nil

	case "file":
		// TODO: Implement file store
		return nil, fmt.Errorf("file store not yet implemented")

	case "redis":
		// TODO: Implement Redis store
		return nil, fmt.Errorf("Redis store not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.Type)
	}
}

// ValidateStoreConfig validates store config
func ValidateStoreConfig(config *StoreConfig) error {
	if config == nil {
		return fmt.Errorf("store configuration cannot be empty")
	}

	if config.Type == "" {
		config.Type = "database" // Default to database store
	}

	supportedTypes := []string{"database", "memory", "file", "redis"}
	isSupported := false
	for _, t := range supportedTypes {
		if config.Type == t {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("unsupported store type: %s, supported types: %v", config.Type, supportedTypes)
	}

	if config.ExpiryHr < 0 {
		return fmt.Errorf("expiry duration cannot be negative")
	}

	return nil
}

// GetDefaultStoreConfig gets default store config
func GetDefaultStoreConfig() *StoreConfig {
	return &StoreConfig{
		Type:     "database",
		Config:   make(map[string]interface{}),
		ExpiryHr: 24,
	}
}
