package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthToken struct {
	secretKey []byte
}

func NewAuthToken(secretKey string) *AuthToken {
	// Validate secret key is not empty
	if secretKey == "" {
		fmt.Println("Error! secret key cannot be empty")
	}
	return &AuthToken{
		secretKey: []byte(secretKey),
	}
}

func (at *AuthToken) GenerateToken(deviceID string) (string, error) {
	// Set expiration to 1 hour later
	expireTime := time.Now().Add(time.Hour)

	// Create claims
	claims := jwt.MapClaims{
		"device_id": deviceID,
		"exp":       expireTime.Unix(),
		"iat":       time.Now().Unix(), // Issued at
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign with secret key
	tokenString, err := token.SignedString(at.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (at *AuthToken) VerifyToken(tokenString string) (bool, string, error) {
	if at == nil {
		return false, "", errors.New("AuthToken instance is nil")
	}

	if at.secretKey == nil {
		return false, "", errors.New("secret key is not initialized")
	}

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return at.secretKey, nil
	})
	if err != nil {
		return false, "", fmt.Errorf("failed to parse token: %w", err)
	}

	// Verify token validity
	if !token.Valid {
		return false, "", errors.New("invalid token")
	}

	// Get claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, "", errors.New("invalid claims")
	}

	// Get device ID
	deviceID, ok := claims["device_id"].(string)
	if !ok {
		return false, "", errors.New("invalid device_id in claims")
	}

	return true, deviceID, nil
}
