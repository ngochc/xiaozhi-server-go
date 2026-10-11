package store

import (
	"time"
)

// ClientInfo client authentication info struct
type ClientInfo struct {
	ClientID  string                 `json:"client_id"`
	Username  string                 `json:"username"`
	Password  string                 `json:"password"`
	IP        string                 `json:"ip"`
	DeviceID  string                 `json:"device_id"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AuthStore authentication store interface
type AuthStore interface {
	// StoreAuth stores client authentication info
	StoreAuth(clientID, username, password string, metadata map[string]interface{}) error

	// ValidateAuth validates client authentication info
	ValidateAuth(clientID, username, password string) (bool, *ClientInfo, error)

	// GetClientInfo gets client information
	GetClientInfo(clientID string) (*ClientInfo, error)

	// RemoveAuth removes client authentication info
	RemoveAuth(clientID string) error

	// ListClients lists all client IDs
	ListClients() ([]string, error)

	// CleanupExpired cleans up expired authentications
	CleanupExpired() error

	// Close closes store connection
	Close() error
}

// StoreConfig store configuration
type StoreConfig struct {
	Type     string                 `yaml:"type"`   // memory/file/redis
	Config   map[string]interface{} `yaml:"config"` // Specific store configuration
	ExpiryHr int                    `yaml:"expiry"` // Expiry time (hours)
}
