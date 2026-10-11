package webapi

import (
	"fmt"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("xiaozhi_jwt_secret")

// currentUserID keeps the community edition single-user. Database user_id
// fields are retained only for schema compatibility and always use this value.
func currentUserID(_ *gin.Context) uint {
	return database.AdminUserID
}

// General authentication middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apikey := c.GetHeader("AuthorToken")
		if apikey != "" {
			// Verify API Token if provided
			if apikey != configs.Cfg.Server.Token {
				utils.DefaultLogger.Error("Invalid API Token %s", apikey)
			} else {
				user, err := database.GetUserByID(database.GetDB(), database.AdminUserID)
				if err != nil || user == nil {
					utils.DefaultLogger.Error("API Token validated, but default user does not exist")
					c.JSON(401, gin.H{"status": "error", "message": "Default user does not exist"})
					c.Abort()
					return
				}
				c.Set("user_id", database.AdminUserID)
				c.Set("username", user.Username)
				c.Next()
				return
			}
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			utils.DefaultLogger.Error("Authentication token missing")
			c.JSON(401, gin.H{"status": "error", "message": "Authentication token missing"})
			c.Abort()
			return
		}
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		claims, err := VerifyJWT(token)
		if err != nil {
			utils.DefaultLogger.Error("Invalid token: %v", err)
			c.JSON(401, gin.H{"status": "error", "message": "Invalid token"})
			c.Abort()
			return
		}
		// Token is used for authentication only; personal edition does not isolate resources by user ID in the token.
		c.Set("user_id", currentUserID(c))
		c.Set("username", claims.Username)
		c.Next()
	}
}

// Admin permission middleware
// Allow observer role read-only access for GET requests; non-GET requests still require admin
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Assume user_id is already set by authentication middleware
		userID, exists := c.Get("user_id")
		if !exists {
			fmt.Println("Unauthenticated")
			c.JSON(401, gin.H{"status": "error", "message": "Unauthenticated"})
			c.Abort()
			return
		}
		user, err := database.GetUserByID(database.GetDB(), userID.(uint))
		if err != nil || user == nil {
			fmt.Println("Permission denied, user information not found")
			c.JSON(403, gin.H{"status": "error", "message": "Permission denied"})
			c.Abort()
			return
		}
		// Allow read-only observer access to GET requests, but non-GET methods still require admin permissions
		method := c.Request.Method
		if method == "GET" {
			if user.Role == "admin" || user.Role == "observer" {
				c.Next()
				return
			}
			fmt.Println("Permission denied, must be admin or observer account (read-only)")
			c.JSON(403, gin.H{"status": "error", "message": "Please use an admin or observer account to view"})
			c.Abort()
			return
		}
		// Non-GET methods only allow admin
		if user.Role != "admin" {
			fmt.Println("Permission denied, must be admin account")
			c.JSON(403, gin.H{"status": "error", "message": "Please use an admin account to perform this operation"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// Generate JWT
func GenerateJWT(userID uint, username string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Validate JWT
func VerifyJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&JWTClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrInvalidKey
}
