package webapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DefaultUserService struct {
	logger *utils.Logger
	config *configs.Config
}

// NewDefaultUserService constructor
func NewDefaultUserService(
	config *configs.Config,
	logger *utils.Logger,
) (*DefaultUserService, error) {
	service := &DefaultUserService{
		logger: logger,
		config: config,
	}
	return service, nil
}

// Start registers User service routes
func (s *DefaultUserService) Start(
	ctx context.Context,
	engine *gin.Engine,
	apiGroup *gin.RouterGroup,
) error {
	apiGroup.POST("/user/login", s.handleLogin)
	apiGroup.POST("/user/logout", s.handleLogout)

	// User routes requiring authentication
	authGroup := apiGroup.Group("/user")
	authGroup.Use(AuthMiddleware())
	{
		authGroup.GET("/profile", s.handleGetProfile)
		authGroup.PUT("/profile", s.handleUpdateProfile)
		authGroup.POST("/change-password", s.handleChangePassword)

		authGroup.GET("/summary", s.handleSystemSummary) // Retrieve user summary metrics

		authGroup.POST("/agent/create", s.handleAgentCreate)
		authGroup.GET("/agent/list", s.handleAgentList)
		authGroup.GET("/agent/:id", s.handleAgentGet)
		authGroup.PUT("/agent/:id", s.handleAgentUpdate)
		authGroup.DELETE("/agent/:id", s.handleAgentDelete)

		authGroup.POST("/agent/history_dialog_list/:id", s.handleAgentHistoryDialogList)
		authGroup.GET("/agent/history_dialog/:dialog_id", s.handleAgentGetHistoryDialog)
		authGroup.DELETE("/agent/history_dialog/:dialog_id", s.handleAgentDeleteHistoryDialog)

		authGroup.GET("/device/list/:id", s.handleDeviceList)
		authGroup.GET("/device/list", s.handleDeviceListByUser)
		authGroup.POST("/device/bind", s.handleDeviceBind)
		authGroup.GET("/device/:id", s.handleDeviceGet)
		authGroup.PUT("/device/:id", s.handleDeviceUpdate)
		authGroup.DELETE("/device", s.handleDeviceDelete)

		// providers
		authGroup.GET("/providers/:type", s.handleUserProvidersType)
		authGroup.POST("/providers/create", s.handleUserProvidersCreate)
		authGroup.POST("/providers/:type/:name/test", s.handleUserProviderTest)
		///user/providers/{type}/{name} [delete]
		authGroup.DELETE("/providers/:type/:name", s.handleUserProvidersDelete)
		///user/providers/{type}/{name} [put]
		authGroup.PUT("/providers/:type/:name", s.handleUserProvidersUpdate)

		authGroup.DELETE("/device/:id/memory", s.handleDeviceMemoryClear)
	}

	s.logger.Info("User HTTP routes registered successfully")
	return nil
}

func (s *DefaultUserService) handleSystemSummary(c *gin.Context) {
	// Retrieve user summary metrics
	userID, _ := c.Get("user_id")
	_, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	data, _ := database.GetSystemSummary(database.GetDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Retrieved system summary successfully",
		"data": gin.H{
			"totle_users":       data["total_users"],
			"totle_agents":      data["total_agents"],
			"totle_devices":     data["total_devices"],
			"online_users":      data["online_devices"],
			"session_devices":   data["session_devices"],
			"system_memory_use": data["memory_usage"],
			"system_cpu_use":    data["cpu_usage"],
		},
	})
}

// LoginRequest User login request body
// @Description User login parameters
// @Tags User
// @Accept json
// @Produce json
// @Param data body LoginRequest true "Login parameters"
// @Success 200 {object} map[string]interface{} "Token and user information returned on successful login"
// @Router /user/login [post]
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest Change password request body
// @Description Change password parameters
// @Tags User
// @Accept json
// @Produce json
// @Param data body ChangePasswordRequest true "Change password parameters"
// @Success 200 {object} map[string]interface{} "Change password result"
// @Router /user/change-password [post]
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// handleLogin User login
// @Summary User login
// @Description User login endpoint
// @Tags User
// @Accept json
// @Produce json
// @Param data body LoginRequest true "Login parameters"
// @Success 200 {object} map[string]interface{} "Token and user information returned on successful login"
// @Router /user/login [post]
func (s *DefaultUserService) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Get user information
	user, err := database.GetUserByUsername(database.GetDB(), req.Username)
	if err != nil || user == nil {
		c.JSON(401, gin.H{
			"status":  "error",
			"message": "Incorrect username or password",
		})
		return
	}

	// Verify password
	if !s.verifyPassword(req.Password, user.Password) {
		c.JSON(401, gin.H{
			"status":  "error",
			"message": "Incorrect username or password",
		})
		return
	}

	// Check user status
	if user.Status != 1 {
		c.JSON(401, gin.H{
			"status":  "error",
			"message": "Account has been disabled",
		})
		return
	}

	// Generate JWT token
	token, err := GenerateJWT(user.ID, user.Username)
	if err != nil {
		s.logger.Error("Failed to generate JWT: %v", err)
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Login failed",
		})
		return
	}

	// Update last login time
	database.UpdateUserLastLogin(database.GetDB(), user.ID)

	s.logger.Info("User logged in successfully: %s", req.Username)
	role := user.Role
	// If admin, assign role field
	data := gin.H{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
	}

	// If admin, assign role field
	if role != "user" {
		data["role"] = role
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"success": true,
		"message": "Login successful",
		"data":    data,
	})
}

// handleLogout User logout
// @Summary User logout
// @Description User logout endpoint
// @Tags User
// @Produce json
// @Success 200 {object} map[string]interface{} "Logout result"
// @Router /user/logout [post]
func (s *DefaultUserService) handleLogout(c *gin.Context) {
	// Token blacklist mechanism can be implemented here
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Logout successful",
	})
}

// handleGetProfile Get user profile
// @Summary Get user profile
// @Description Get personal information of currently logged in user
// @Tags User
// @Produce json
// @Success 200 {object} map[string]interface{} "User profile"
// @Router /user/profile [get]
func (s *DefaultUserService) handleGetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "User profile retrieved successfully",
		"data": gin.H{
			"user_id":    user.ID,
			"username":   user.Username,
			"nickname":   user.Nickname,
			"head_img":   user.HeadImg,
			"email":      user.Email,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
		},
	})
}

// handleUpdateProfile Update user profile
// @Summary Update user profile
// @Description Update personal information of currently logged in user
// @Tags User
// @Accept json
// @Produce json
// @Param data body object true "User profile parameters"
// @Success 200 {object} map[string]interface{} "Updated user profile"
// @Router /user/profile [put]
func (s *DefaultUserService) handleUpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var updateData struct {
		Nickname string `json:"nickname" binding:"omitempty,min=3,max=20"`
		Email    string `json:"email" binding:"omitempty,email"`
		HeadImg  string `json:"head_img" binding:"omitempty,url"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		user, err := database.GetUserByID(tx, userID.(uint))
		if err != nil {
			c.JSON(404, gin.H{
				"status":  "error",
				"message": "User does not exist",
			})
			return err
		}
		if updateData.Nickname != user.Nickname {
			err := database.UpdateUserNickname(tx, userID.(uint), updateData.Nickname)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": "Failed to update nickname",
					"error":   err.Error(),
				})
				return err
			}
			user.Nickname = updateData.Nickname
		}
		if updateData.HeadImg != "" && updateData.HeadImg != user.HeadImg {
			err := database.UpdateUserHeadImg(tx, userID.(uint), updateData.HeadImg)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": "Failed to update avatar",
					"error":   err.Error(),
				})
				return err
			}
			user.HeadImg = updateData.HeadImg
		}
		if updateData.Email != user.Email {
			err := database.UpdateUserEmail(tx, userID.(uint), updateData.Email)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": "Failed to update email",
					"error":   err.Error(),
				})
				return err
			}
			user.Email = updateData.Email
		}
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Updated successfully",
		})
		return nil
	})
}

// handleChangePassword Change password
// @Summary Change password
// @Description Change password of currently logged in user
// @Tags User
// @Accept json
// @Produce json
// @Param data body ChangePasswordRequest true "Change password parameters"
// @Success 200 {object} map[string]interface{} "Change password result"
// @Router /user/change-password [post]
func (s *DefaultUserService) handleChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Get user information to verify old password
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	// Verify old password
	if !s.verifyPassword(req.OldPassword, user.Password) {
		s.logger.Error(
			"User password change failed: incorrect old password: %s, old:%s, req:%s",
			user.Username,
			user.Password,
			req.OldPassword,
		)
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Incorrect old password",
		})
		return
	}

	// Update password
	hashedPassword := s.hashPassword(req.NewPassword)
	if err := database.UpdateUserPassword(database.GetDB(), userID.(uint), hashedPassword); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to change password",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Password changed successfully",
	})
}

// Password hash
func (s *DefaultUserService) hashPassword(password string) string {
	hash := md5.Sum([]byte(password + "xiaozhi_salt"))
	return hex.EncodeToString(hash[:])
}

// Verify password
func (s *DefaultUserService) verifyPassword(password, hashedPassword string) bool {
	return s.hashPassword(password) == hashedPassword
}
