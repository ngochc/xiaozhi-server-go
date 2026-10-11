package models

import (
	"time"

	"gorm.io/datatypes"
)

/*
Used to store client authentication credentials.
Distributed via OTA request; used for client authentication after connecting.
*/

type AuthClient struct {
	ID        uint           `gorm:"primaryKey"`
	ClientID  string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"client_id"`
	Username  string         `gorm:"not null"                               json:"username"`
	Password  string         `gorm:"not null"                               json:"password"`
	IP        string         `                                              json:"ip"`
	DeviceID  string         `                                              json:"device_id"`
	CreatedAt time.Time      `                                              json:"created_at"`
	ExpiresAt *time.Time     `                                              json:"expires_at,omitempty"`
	Metadata  datatypes.JSON `                                              json:"metadata,omitempty"`
}
