package models

import "gorm.io/datatypes"

type ASRConfig struct {
	ID     uint           `gorm:"primaryKey" json:"-"`
	UserID uint           `gorm:"default:1" json:"-"`
	Name   string         `                  json:"name"`
	Type   string         `                  json:"type"` // ASR type
	Data   datatypes.JSON `                  json:"data"` // Configuration data
}

type LLMConfig struct {
	ID     uint           `gorm:"primaryKey" json:"-"`
	UserID uint           `gorm:"default:1" json:"-"`
	Name   string         `                  json:"name"`
	Type   string         `                  json:"type"`
	Data   datatypes.JSON `                  json:"data"` // Configuration data
}

type TTSConfig struct {
	ID     uint           `gorm:"primaryKey" json:"-"`
	UserID uint           `gorm:"default:1" json:"-"`
	Name   string         `                  json:"name"`
	Type   string         `                  json:"type"`
	Data   datatypes.JSON `                  json:"data"` // Configuration data
}

type VLLLMConfig struct {
	ID     uint           `gorm:"primaryKey" json:"-"`
	UserID uint           `gorm:"default:1" json:"-"`
	Name   string         `                  json:"name"` // Provider name
	Type   string         `                  json:"type"`
	Data   datatypes.JSON `                  json:"data"` // Configuration data
}
