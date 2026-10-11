package database

import (
	"math"
	"strconv"
	"time"
	"xiaozhi-server-go/src/models"
)

func GetServerStatus() (*models.ServerStatus, error) {
	var status models.ServerStatus
	// Uses fixed ID (ServerStatusID) to maintain a single record
	if err := GetDB().FirstOrCreate(&status, models.ServerStatus{ID: ServerStatusID}).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

// UpdateServerStatus updates server status metrics for a single record
func UpdateServerStatus(CPUUsage float64, memoryUsage float64, onlineDeviceNum int, onlineSessionNum int) error {
	CPUUsageStr := strconv.FormatFloat(math.Round(CPUUsage*100)/100, 'f', 2, 64)
	memoryUsageStr := strconv.FormatFloat(math.Round(memoryUsage*100)/100, 'f', 2, 64)

	status := models.ServerStatus{
		ID:               ServerStatusID,
		CPUUsage:         CPUUsageStr,
		MemoryUsage:      memoryUsageStr,
		OnlineDeviceNum:  onlineDeviceNum,
		OnlineSessionNum: onlineSessionNum,
		UpdatedAt:        time.Now(),
	}

	return GetDB().Save(&status).Error
}
