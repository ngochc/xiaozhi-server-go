package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent struct: belongs to a user and can bind multiple devices.
type Agent struct {
	ID                 uint      `gorm:"primaryKey"           json:"id"`
	Name               string    `gorm:"not null"             json:"name"` // Agent name
	LLM                string    `gorm:"default:'ChatGLMLLM'" json:"LLM"`
	Language           string    `gorm:"default:'English'"    json:"language"`                            // Language
	Voice              string    `gorm:"default:'en-US-AriaNeural'"       json:"voice"`                   // Voice identifier
	VoiceName          string    `gorm:"default:'Aria'"       json:"voiceName"`                           // Voice display name
	Prompt             string    `gorm:"type:text"            json:"prompt"`
	ASRSpeed           int       `gorm:"default:2"            json:"asrSpeed"`           // ASR speed: 1=patient, 2=normal, 3=fast
	SpeakSpeed         int       `gorm:"default:2"            json:"speakSpeed"`         // TTS speed: 1=slow, 2=normal, 3=fast
	Tone               int       `gorm:"default:50"           json:"tone"`               // TTS pitch: 1-100, low to high
	UserID             uint      `gorm:"not null;default:1"   json:"-"`                  // User ownership ID
	CreatedAt          time.Time `                            json:"createdAt"`          // Created timestamp
	UpdatedAt          time.Time `                            json:"updatedAt"`          // Updated timestamp
	LastConversationAt time.Time `                            json:"lastConversationAt"` // Last conversation timestamp
	Devices            []Device  `gorm:"foreignKey:AgentID"   json:"-"`                  // Associated devices
	EnabledTools       string    `gorm:"type:text"            json:"enabledTools"`       // Enabled tools list e.g. "tool1,tool2"
	Conversationid     string    `                            json:"conversationId"`     // Associated AgentDialog conversation ID
	HeadImg            string    `gorm:"type:varchar(255)"    json:"head_img"`           // Avatar URL
	Description        string    `gorm:"type:text"            json:"description"`        // Agent description
	CatalogyID         uint      `                            json:"catalogy_id"`        // Category ID
	Extra              string    `gorm:"type:text"            json:"extra"`              // Extra JSON metadata
	// IsOnboarding marks system default Setup Assistant.
	IsOnboarding bool `gorm:"default:false;index"  json:"isOnboarding"`
}

type AgentDialog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Conversationid string    `                  json:"conversationId"`
	AgentID        uint      `gorm:"index"      json:"agentID"`          // Associated Agent foreign key
	UserID         uint      `gorm:"index;default:1" json:"userID"`      // User ID
	Dialog         string    `gorm:"type:text"  json:"dialog,omitempty"` // Conversation content
	CreatedAt      time.Time `                  json:"createdAt"`        // Created timestamp
	UpdatedAt      time.Time `                  json:"updatedAt"`        // Updated timestamp
}

type Device struct {
	ID               uint           `gorm:"primaryKey"                             json:"id"`
	AgentID          *uint          `gorm:"index"                                  json:"agentID"`          // Associated Agent foreign key
	UserID           *uint          `gorm:"index;default:1"                        json:"userID"`           // User ID
	Name             string         `gorm:"not null"                               json:"name"`             // Device name
	DeviceID         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"deviceId"`         // Device unique ID / MAC
	MACAddress       string         `gorm:"type:varchar(17);index"                json:"macAddress"`        // Physical MAC address (AA:BB:CC:DD:EE:FF)
	ClientID         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"clientId"`         // Client unique ID
	Version          string         `                                              json:"version"`          // Firmware version
	OTA              bool           `gorm:"default:true"                           json:"ota"`              // OTA upgrade support
	RegisterTime     int64          `                                              json:"-"`                // Registration timestamp
	LastActiveTime   int64          `                                              json:"-"`                // Last active timestamp
	RegisterTimeV2   time.Time      `                                              json:"registerTimeV2"`   // Registration time
	LastActiveTimeV2 time.Time      `                                              json:"lastActiveTimeV2"` // Last active time
	Online           bool           `                                              json:"online"`           // Online status
	AuthCode         string         `                                              json:"authCode"`         // Auth verification code
	AuthStatus       string         `                                              json:"authStatus"`       // Auth status (e.g. pending)
	BoardType        string         `                                              json:"boardType"`        // Board type
	ChipModelName    string         `                                              json:"chipModelName"`    // Chip model name (default: esp32s3)
	Channel          int            `                                              json:"channel"`          // WiFi channel
	SSID             string         `                                              json:"ssid"`             // WiFi SSID
	Application      string         `                                              json:"application"`      // Application name
	Language         string         `gorm:"default:'en-US'"                        json:"language"`         // Language
	DeviceCode       string         `                                              json:"deviceCode"`       // Device code
	DeletedAt        gorm.DeletedAt `gorm:"index"                                  json:"-"`                // Soft delete
	Extra            string         `gorm:"type:text"                              json:"extra"`            // Extra JSON metadata
	Conversationid   string         `                                              json:"conversationId"`   // Associated conversation ID
	Mode             string         `                                              json:"mode"`             // Mode: chat/listen/ban
}

// User model
type User struct {
	ID          uint      `gorm:"primaryKey"                             json:"id"`
	Username    string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"username"`
	Password    string    `                                              json:"-"`        // Password hash
	Nickname    string    `gorm:"type:varchar(255)"                      json:"nickname"` // Nickname
	HeadImg     string    `gorm:"type:varchar(255)"                      json:"head_img"` // Avatar URL
	Role        string    `                                              json:"role"`     // Role: admin/observer/user
	CreatedAt   time.Time `                                              json:"createdAt"`
	UpdatedAt   time.Time `                                              json:"updatedAt"`
	Email       string    `gorm:"type:varchar(255);uniqueIndex;"         json:"email"`  // User email
	Status      uint      `gorm:"default:1"                              json:"status"` // Status: 1=active, 0=disabled
	PhoneNumber string    `gorm:"type:varchar(20);" json:"phoneNumber"`                 // Phone number
	Extra       string    `gorm:"type:text"                              json:"extra"`  // Extra JSON metadata
}

type ServerConfig struct {
	ID     uint   `gorm:"primaryKey"`
	CfgStr string `gorm:"type:text"` // Server config content serialized from config.yaml
}

type ServerStatus struct {
	ID               uint      `gorm:"primaryKey"`
	OnlineDeviceNum  int       `json:"onlineDeviceNum"`  // Real-time online devices count
	OnlineSessionNum int       `json:"onlineSessionNum"` // Active talking sessions count
	CPUUsage         string    `json:"cpu_usage"`
	MemoryUsage      string    `json:"memory_usage"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
