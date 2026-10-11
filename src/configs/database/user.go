package database

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

var (
	AdminUserID uint = 1
)

const initialAdminPasswordLength = 16

const initialAdminPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// NormalizeSingleUserData converts historical multi-user ownership records to
// the single local account used by the personal open-source edition.
func NormalizeSingleUserData(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		updates := []struct {
			model interface{}
			where string
		}{
			{&models.Agent{}, "user_id <> ?"},
			{&models.AgentDialog{}, "user_id <> ?"},
			{&models.Device{}, "user_id IS NULL OR user_id <> ?"},
			{&models.ASRConfig{}, "user_id <> ?"},
			{&models.TTSConfig{}, "user_id <> ?"},
			{&models.LLMConfig{}, "user_id <> ?"},
			{&models.VLLLMConfig{}, "user_id <> ?"},
		}
		for _, item := range updates {
			if err := tx.Model(item.model).Where(item.where, AdminUserID).Update("user_id", AdminUserID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func generateInitialAdminPassword() (string, error) {
	password := make([]byte, initialAdminPasswordLength)
	for i := range password {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(initialAdminPasswordAlphabet))))
		if err != nil {
			return "", fmt.Errorf("failed to generate initial admin password: %w", err)
		}
		password[i] = initialAdminPasswordAlphabet[index.Int64()]
	}
	return string(password), nil
}

func InitAdminUser(db *gorm.DB, config *configs.Config) error {
	var count int64
	// Check if admin user already exists
	if err := db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password, err := generateInitialAdminPassword()
	if err != nil {
		return err
	}
	hash := md5.Sum([]byte(password + "xiaozhi_salt"))
	pwd := hex.EncodeToString(hash[:])
	// Create admin user
	adminUser := &models.User{
		ID:        AdminUserID,
		Username:  "admin",
		Email:     "",
		Password:  pwd,
		Role:      "admin",
		Status:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(adminUser).Error; err != nil {
		return err
	}
	fmt.Printf("Initial start: created local admin user 'admin' with temporary password: %s. Please change it immediately; this password is only shown in this startup log.\n", password)

	return nil
}

// GetUserByUsername retrieves a user by username
func GetUserByUsername(tx *gorm.DB, username string) (*models.User, error) {
	var user models.User
	err := tx.Where("username = ?", username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(tx *gorm.DB, userID uint) (*models.User, error) {
	var user models.User
	err := tx.Where("id = ?", userID).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUserLastLogin updates the user's last login timestamp
func UpdateUserLastLogin(tx *gorm.DB, userID uint) error {
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"updated_at": time.Now(),
		}).Error
	return err
}

// UpdateUserEmail updates the user's email
func UpdateUserEmail(tx *gorm.DB, userID uint, email string) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	var existingUser models.User
	tx.Where("email = ?", email).First(&existingUser)
	if existingUser.ID != 0 && existingUser.ID != userID {
		return fmt.Errorf("email is already in use by another user")
	}

	if email != "" {
		updates["email"] = email
	}
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
	return err
}

func UpdateUserName(tx *gorm.DB, userID uint, username string) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	var existingUser models.User
	tx.Where("username = ?", username).First(&existingUser)
	if existingUser.ID != 0 && existingUser.ID != userID {
		return fmt.Errorf("username is already in use by another user")
	}

	if username != "" {
		updates["username"] = username
	}
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
	return err
}

func UpdateUserNickname(tx *gorm.DB, userID uint, nickname string) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
	return err
}

func UpdateUserHeadImg(tx *gorm.DB, userID uint, headImg string) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if headImg != "" {
		updates["head_img"] = headImg
	}
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
	return err
}

// UpdateUserPassword updates the user's password
func UpdateUserPassword(tx *gorm.DB, userID uint, hashedPassword string) error {
	err := tx.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		}).Error
	return err
}

// GetSystemSummary retrieves system resource summary
func GetSystemSummary(tx *gorm.DB) (map[string]interface{}, error) {
	status, err := GetServerStatus()
	if err != nil {
		return nil, err
	}
	cpuUsage := status.CPUUsage
	memoryUsage := status.MemoryUsage

	var agentCount int64
	if err := tx.Model(&models.Agent{}).Count(&agentCount).Error; err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"total_agents": agentCount,
		"cpu_usage":    cpuUsage,
		"memory_usage": memoryUsage,
	}
	return summary, nil
}
