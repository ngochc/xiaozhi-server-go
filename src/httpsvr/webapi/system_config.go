package webapi

import (
	"encoding/json"
	"fmt"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemConfigService handles system configuration
type SystemConfigService struct {
	logger *utils.Logger
	db     *gorm.DB
}

// NewSystemConfigService constructor
func NewSystemConfigService(logger *utils.Logger, db *gorm.DB) *SystemConfigService {
	return &SystemConfigService{
		logger: logger,
		db:     db,
	}
}

// RegisterRoutes registers configuration routes
func (s *SystemConfigService) RegisterRoutes(apiGroup *gin.RouterGroup) {
	// Configuration routes requiring admin permissions
	adminGroup := apiGroup.Group("/admin/config")
	adminGroup.Use(AuthMiddleware(), AdminMiddleware())
	{
		// Application configuration
		adminGroup.GET("/application", s.handleGetApplicationConfig)
		adminGroup.PUT("/application", s.handleUpdateApplicationConfig)

		// Auth configuration
		adminGroup.GET("/auth", s.handleGetAuthConfig)
		adminGroup.PUT("/auth", s.handleUpdateAuthConfig)

		// Transport configuration
		adminGroup.GET("/transport", s.handleGetTransportConfig)
		adminGroup.PUT("/transport", s.handleUpdateTransportConfig)

		// Web configuration
		adminGroup.GET("/web", s.handleGetWebConfig)
		adminGroup.PUT("/web", s.handleUpdateWebConfig)
		adminGroup.POST("/diagnostics", s.handleRunDiagnostics)

		// Logging configuration
		adminGroup.GET("/log", s.handleGetLogConfig)
		adminGroup.PUT("/log", s.handleUpdateLogConfig)

		// Roles configuration
		adminGroup.GET("/roles", s.handleGetRoleConfigs)
		adminGroup.POST("/roles", s.handleCreateRoleConfig)
		adminGroup.PUT("/roles/:id", s.handleUpdateRoleConfig)
		adminGroup.DELETE("/roles/:id", s.handleDeleteRoleConfig)

		// Quick reply words
		adminGroup.GET("/quick-reply", s.handleGetQuickReplyWords)
		adminGroup.POST("/quick-reply", s.handleCreateQuickReplyWord)
		adminGroup.PUT("/quick-reply/:id", s.handleUpdateQuickReplyWord)
		adminGroup.DELETE("/quick-reply/:id", s.handleDeleteQuickReplyWord)

		// Local MCP tools
		adminGroup.GET("/mcp-functions", s.handleGetMCPFunctions)
		adminGroup.POST("/mcp-functions", s.handleCreateMCPFunction)
		adminGroup.PUT("/mcp-functions/:id", s.handleUpdateMCPFunction)
		adminGroup.DELETE("/mcp-functions/:id", s.handleDeleteMCPFunction)

		// Exit commands
		adminGroup.GET("/exit-commands", s.handleGetExitCommands)
		adminGroup.POST("/exit-commands", s.handleCreateExitCommand)
		adminGroup.PUT("/exit-commands/:id", s.handleUpdateExitCommand)
		adminGroup.DELETE("/exit-commands/:id", s.handleDeleteExitCommand)
	}
}

type ApplicationConfig struct {
	EnableMCPFilter bool `json:"enableMCPFilter"`
	QuickReply      bool `json:"quickReply"`
	SaveTtsAudio    bool `json:"saveTtsAudio"`
	SaveUserAudio   bool `json:"saveUserAudio"`
}

// Application configuration handlers
func (s *SystemConfigService) handleGetApplicationConfig(c *gin.Context) {
	var config ApplicationConfig
	config.SaveTtsAudio = configs.Cfg.SaveTTSAudio
	config.SaveUserAudio = configs.Cfg.SaveUserAudio
	config.QuickReply = configs.Cfg.QuickReply

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   config,
	})
}

func (s *SystemConfigService) handleUpdateApplicationConfig(c *gin.Context) {
	var config ApplicationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	configs.Cfg.SaveTTSAudio = config.SaveTtsAudio
	configs.Cfg.SaveUserAudio = config.SaveUserAudio
	configs.Cfg.QuickReply = config.QuickReply
	fmt.Println("Setting application config:", config)
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Application configuration updated successfully",
		"data":    config,
	})
}

type AuthConfig struct {
	Token  string `json:"token"`
	Expiry int    `json:"expiry"`
}

// Auth configuration handlers
func (s *SystemConfigService) handleGetAuthConfig(c *gin.Context) {
	var config AuthConfig
	config.Token = configs.Cfg.Server.Token
	config.Expiry = configs.Cfg.Server.Auth.Store.Expiry

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   config,
	})
}

func (s *SystemConfigService) handleUpdateAuthConfig(c *gin.Context) {
	var config AuthConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}
	configs.Cfg.Server.Token = config.Token
	configs.Cfg.Server.Auth.Store.Expiry = config.Expiry
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Authentication configuration updated successfully",
		"data":    config,
	})
}

type TransportConfig struct {
	Type    string          `json:"type"` // websocket/mqtt_udp
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"` // arbitrary JSON payload
}

// Transport configuration handlers
func (s *SystemConfigService) handleGetTransportConfig(c *gin.Context) {
	var configsTransport []TransportConfig
	configsTransport = make([]TransportConfig, 0)
	str, _ := json.Marshal(configs.Cfg.Transport.WebSocket)
	configsTransport = append(configsTransport, TransportConfig{
		Type:    "websocket",
		Enabled: configs.Cfg.Transport.WebSocket.Enabled,
		Config:  str,
	})
	fmt.Println("Get WebSocket config:", configs.Cfg.Transport.WebSocket.Enabled)

	strMqttUdp, _ := json.Marshal(configs.Cfg.Transport.MQTTUDP)
	configsTransport = append(configsTransport, TransportConfig{
		Type:    "mqtt_udp",
		Enabled: configs.Cfg.Transport.MQTTUDP.Enabled,
		Config:  strMqttUdp,
	})
	c.JSON(200, gin.H{
		"status": "ok",
		"data":   configsTransport,
	})
}

func (s *SystemConfigService) handleUpdateTransportConfig(c *gin.Context) {
	var configsTransport []TransportConfig
	if err := c.ShouldBindJSON(&configsTransport); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		fmt.Println("Failed to bind transport config:", err)
		return
	}
	fmt.Println("Updating transport config:", configsTransport)

	for _, cfg := range configsTransport {
		// Update transport configuration
		switch cfg.Type {
		case "websocket":
			configs.Cfg.Transport.WebSocket.Enabled = cfg.Enabled
			fmt.Println("Update WebSocket config:", (cfg.Enabled))
			json.Unmarshal(cfg.Config, &configs.Cfg.Transport.WebSocket)
			fmt.Println("Update WebSocket config content:", configs.Cfg.Transport.WebSocket)
		case "mqtt_udp":
			configs.Cfg.Transport.MQTTUDP.Enabled = cfg.Enabled
			json.Unmarshal(cfg.Config, &configs.Cfg.Transport.MQTTUDP)
		}
	}
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Transport configuration updated successfully",
		"data":    configsTransport,
	})
}

// WebConfig Web UI configuration
type WebConfig struct {
	Enabled      bool   `json:"enabled"`
	Port         int    `json:"port"`
	StaticDir    string `json:"staticDir"`
	Websocket    string `json:"websocket"`
	VisionURL    string `json:"visionUrl"`
	ActivateText string `json:"activateText"` // Text sent with activation code
}

// LogConfig logging configuration
type LogConfig struct {
	LogLevel string `json:"logLevel"`
	LogDir   string `json:"logDir"`
	LogFile  string `json:"logFile"`
}

// RoleConfig role configuration
type RoleConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// QuickReplyWord Quick reply words
type QuickReplyWord struct {
	Word    string `json:"word"`
	Enabled bool   `json:"enabled"`
	Order   int    `json:"order"`
}

// LocalMCPFunction local MCP tools configuration
type LocalMCPFunction struct {
	FunctionName string `json:"functionName"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
}

// ExitCommand Exit commands
type ExitCommand struct {
	Command string `json:"command"`
	Enabled bool   `json:"enabled"`
}

// Web configuration handlers (reads/saves via configs.Cfg)
func (s *SystemConfigService) handleGetWebConfig(c *gin.Context) {
	// Construct response DTO from global configuration
	config := WebConfig{
		Port:         configs.Cfg.Web.Port,
		StaticDir:    configs.Cfg.Web.StaticDir,
		Websocket:    configs.Cfg.Web.Websocket,
		VisionURL:    configs.Cfg.Web.VisionURL,
		ActivateText: configs.Cfg.Web.ActivateText,
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   config,
	})
}

func (s *SystemConfigService) handleUpdateWebConfig(c *gin.Context) {
	var payload WebConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Apply DTO to global configuration and save to database
	configs.Cfg.Web.Port = payload.Port
	configs.Cfg.Web.StaticDir = payload.StaticDir
	configs.Cfg.Web.Websocket = payload.Websocket
	configs.Cfg.Web.VisionURL = payload.VisionURL
	configs.Cfg.Web.ActivateText = payload.ActivateText

	if err := configs.Cfg.SaveToDB(database.GetServerConfigDB()); err != nil {
		s.logger.Error("Failed to save Web configuration to database: %v", err)
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to save Web configuration",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Web configuration updated successfully",
		"data":    payload,
	})
}

// Logging configuration handlers
func (s *SystemConfigService) handleGetLogConfig(c *gin.Context) {
	// Read from global configuration and return DTO
	cfg := LogConfig{
		LogLevel: configs.Cfg.Log.LogLevel,
		LogDir:   configs.Cfg.Log.LogDir,
		LogFile:  configs.Cfg.Log.LogFile,
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   cfg,
	})
}

func (s *SystemConfigService) handleUpdateLogConfig(c *gin.Context) {
	var payload LogConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Apply to global configuration and save
	configs.Cfg.Log.LogLevel = payload.LogLevel
	configs.Cfg.Log.LogDir = payload.LogDir
	configs.Cfg.Log.LogFile = payload.LogFile

	if err := configs.Cfg.SaveToDB(database.GetServerConfigDB()); err != nil {
		s.logger.Error("Failed to save logging configuration to database: %v", err)
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to save logging configuration",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Logging configuration updated successfully",
		"data":    payload,
	})
}

// Roles configuration handlers
func (s *SystemConfigService) handleGetRoleConfigs(c *gin.Context) {
	var roles []RoleConfig
	for _, role := range configs.Cfg.Roles {
		roles = append(roles, RoleConfig{
			Name:        role.Name,
			Description: role.Description,
			Enabled:     role.Enabled,
		})
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   roles,
	})
}

func (s *SystemConfigService) handleCreateRoleConfig(c *gin.Context) {
	var config RoleConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}
	configs.Cfg.Roles = append(configs.Cfg.Roles, configs.Role{
		Name:        config.Name,
		Description: config.Description,
		Enabled:     config.Enabled,
	})
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Role configuration created successfully",
		"data":    config,
	})
}

func (s *SystemConfigService) handleUpdateRoleConfig(c *gin.Context) {

	var config RoleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	configs.Cfg.Roles = []configs.Role{}
	configs.Cfg.Roles = append(configs.Cfg.Roles, configs.Role{
		Name:        config.Name,
		Description: config.Description,
		Enabled:     config.Enabled,
	})
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Role configuration updated successfully",
	})
}

func (s *SystemConfigService) handleDeleteRoleConfig(c *gin.Context) {
	name := c.Param("id")
	newRoles := []configs.Role{}
	for _, role := range configs.Cfg.Roles {
		if role.Name != name {
			newRoles = append(newRoles, role)
		}
	}
	configs.Cfg.Roles = newRoles
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Role configuration deleted successfully",
	})
}

// Quick reply words handlers
func (s *SystemConfigService) handleGetQuickReplyWords(c *gin.Context) {
	var words []QuickReplyWord
	words = make([]QuickReplyWord, 0)
	for _, dbWord := range configs.Cfg.QuickReplyWords {
		words = append(words, QuickReplyWord{
			Word:    dbWord,
			Enabled: true,
		})
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   words,
	})
}

func (s *SystemConfigService) handleCreateQuickReplyWord(c *gin.Context) {
	var word QuickReplyWord
	if err := c.ShouldBindJSON(&word); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	configs.Cfg.QuickReplyWords = append(configs.Cfg.QuickReplyWords, word.Word)
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Quick reply words created successfully",
		"data":    word,
	})
}

func (s *SystemConfigService) handleUpdateQuickReplyWord(c *gin.Context) {
	var word QuickReplyWord
	if err := c.ShouldBindJSON(&word); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	for i, dbWord := range configs.Cfg.QuickReplyWords {
		if dbWord == word.Word {
			configs.Cfg.QuickReplyWords[i] = word.Word
			break
		}
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Quick reply words updated successfully",
	})
}

func (s *SystemConfigService) handleDeleteQuickReplyWord(c *gin.Context) {
	word := c.Param("id")
	newWords := []string{}
	for _, dbWord := range configs.Cfg.QuickReplyWords {
		if dbWord != word {
			newWords = append(newWords, dbWord)
		}
	}
	configs.Cfg.QuickReplyWords = newWords
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Quick reply words deleted successfully",
	})
}

// Local MCP tools handlers
func (s *SystemConfigService) handleGetMCPFunctions(c *gin.Context) {
	var functions []LocalMCPFunction
	functions = make([]LocalMCPFunction, 0)
	for _, dbFunc := range configs.Cfg.LocalMCPFun {
		functions = append(functions, LocalMCPFunction{
			FunctionName: dbFunc.Name,
			Description:  dbFunc.Description,
			Enabled:      dbFunc.Enabled,
		})
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   functions,
	})
}

func (s *SystemConfigService) handleCreateMCPFunction(c *gin.Context) {
	var function LocalMCPFunction
	if err := c.ShouldBindJSON(&function); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}
	fmt.Println("Creating MCP function:", function)
	configs.Cfg.LocalMCPFun = append(configs.Cfg.LocalMCPFun, configs.LocalMCPFun{
		Name:        function.FunctionName,
		Description: function.Description,
		Enabled:     function.Enabled,
	})
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "MCP function created successfully",
		"data":    function,
	})
}

func (s *SystemConfigService) handleUpdateMCPFunction(c *gin.Context) {
	id := c.Param("id")
	var function LocalMCPFunction
	if err := c.ShouldBindJSON(&function); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	for i, dbFunc := range configs.Cfg.LocalMCPFun {
		if dbFunc.Name == id {
			configs.Cfg.LocalMCPFun[i] = configs.LocalMCPFun{
				Name:        function.FunctionName,
				Description: function.Description,
				Enabled:     function.Enabled,
			}
		}
	}
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "MCP function updated successfully",
	})
}

func (s *SystemConfigService) handleDeleteMCPFunction(c *gin.Context) {
	id := c.Param("id")
	newFuncs := []configs.LocalMCPFun{}
	for _, dbFunc := range configs.Cfg.LocalMCPFun {
		if dbFunc.Name != id {
			newFuncs = append(newFuncs, dbFunc)
		}
	}
	configs.Cfg.LocalMCPFun = newFuncs
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "MCP function deleted successfully",
	})
}

// Exit commands handlers
func (s *SystemConfigService) handleGetExitCommands(c *gin.Context) {
	var commands []ExitCommand
	commands = make([]ExitCommand, 0)
	for _, dbCmd := range configs.Cfg.CMDExit {
		commands = append(commands, ExitCommand{
			Command: dbCmd,
			Enabled: true,
		})
	}

	c.JSON(200, gin.H{
		"status": "ok",
		"data":   commands,
	})
}

func (s *SystemConfigService) handleCreateExitCommand(c *gin.Context) {
	var command ExitCommand
	if err := c.ShouldBindJSON(&command); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	configs.Cfg.CMDExit = append(configs.Cfg.CMDExit, command.Command)
	configs.Cfg.SaveToDB(database.GetServerConfigDB())

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Exit commands created successfully",
		"data":    command,
	})
}

func (s *SystemConfigService) handleUpdateExitCommand(c *gin.Context) {
	id := c.Param("id")
	var command ExitCommand
	if err := c.ShouldBindJSON(&command); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}
	for i, dbCmd := range configs.Cfg.CMDExit {
		if dbCmd == id {
			configs.Cfg.CMDExit[i] = command.Command
			break
		}
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Exit commands updated successfully",
	})
}

func (s *SystemConfigService) handleDeleteExitCommand(c *gin.Context) {
	id := c.Param("id")
	newCmds := []string{}
	for _, dbCmd := range configs.Cfg.CMDExit {
		if dbCmd != id {
			newCmds = append(newCmds, dbCmd)
		}
	}
	configs.Cfg.CMDExit = newCmds
	configs.Cfg.SaveToDB(database.GetServerConfigDB())
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Exit commands deleted successfully",
	})
}
