package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/types"
	"xiaozhi-server-go/src/core/utils"

	go_openai "github.com/sashabaranov/go-openai"
)

// Conn is connection interface for sending messages
type Conn interface {
	WriteMessage(messageType int, data []byte) error
}

// Manager MCP service manager
type Manager struct {
	logger                *utils.Logger
	conn                  Conn
	funcHandler           types.FunctionRegistryInterface
	configPath            string
	clients               map[string]MCPClient
	localClient           *LocalClient // Local MCP client
	tools                 []string
	XiaoZhiMCPClient      *XiaoZhiMCPClient // XiaoZhiMCPClient handles Xiaozhi MCP protocol
	bRegisteredXiaoZhiMCP bool              // Whether Xiaozhi MCP tools are registered
	isInitialized         bool              // Initialization state flag
	systemCfg             *configs.Config
	mu                    sync.RWMutex
}

// NewManager creates an MCP manager for one connection.
func NewManager(lg *utils.Logger, cfg *configs.Config) *Manager {
	lg.Info("creating MCP Manager")
	projectDir := utils.GetProjectDir()
	configPath := filepath.Join(projectDir, ".mcp_server_settings.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = ""
	}

	mgr := &Manager{
		logger:                lg,
		funcHandler:           nil, // Configured on connection bind
		conn:                  nil, // Configured on connection bind
		configPath:            configPath,
		clients:               make(map[string]MCPClient),
		tools:                 make([]string, 0),
		bRegisteredXiaoZhiMCP: false,
		systemCfg:             cfg,
	}
	// Pre-initialize non-connection MCP servers
	if err := mgr.preInitializeServers(); err != nil {
		if err.Error() == "no valid MCP server configuration found" {
			lg.Warn("No valid MCP server configuration found, skipping pre-initialization")
		} else {
			lg.Error("failed to pre-initialize MCP servers: %v", err)
		}
	}

	return mgr
}

// preInitializeServers pre-initializes external MCP servers
func (m *Manager) preInitializeServers() error {
	m.localClient, _ = NewLocalClient(m.logger, m.systemCfg)
	m.localClient.Start(context.Background())
	m.clients["local"] = m.localClient

	config := m.LoadConfig()
	if config == nil {
		// Skip external server initialization if no config
		m.logger.Info("MCP server configuration not found, skipping external MCP server initialization")
		m.isInitialized = true
		return nil
	}

	for name, srvConfig := range config {
		// Initialize external MCP servers that do not require connection
		srvConfigMap, ok := srvConfig.(map[string]interface{})

		if !ok {
			m.logger.Warn("Invalid configuration format for server %s", name)
			continue
		}

		// Create and start external MCP clients
		clientConfig, err := convertConfig(srvConfigMap)
		if err != nil {
			m.logger.Error("Failed to convert config for server %s: %v", name, err)
			continue
		}

		if !clientConfig.Enabled {
			m.logger.Debug("MCP client %s is disabled", name)
			continue
		}

		client, err := NewClient(clientConfig, m.logger)
		if err != nil {
			m.logger.Error("Failed to create MCP client for server %s: %v", name, err)
			continue
		}

		if err := client.Start(context.Background()); err != nil {
			m.logger.Error("Failed to start MCP client %s: %v", name, err)
			continue
		}
		m.clients[name] = client
	}

	m.isInitialized = true
	return nil
}

func (m *Manager) GetAllToolsNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, len(m.tools))
	copy(names, m.tools)
	return names
}

// BindConnection binds connection to MCP Manager
func (m *Manager) BindConnection(
	conn Conn,
	fh types.FunctionRegistryInterface,
	params interface{},
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conn = conn
	m.funcHandler = fh
	paramsMap := params.(map[string]interface{})
	sessionID := paramsMap["session_id"].(string)
	visionURL := paramsMap["vision_url"].(string)
	deviceID := paramsMap["device_id"].(string)
	clientID := paramsMap["client_id"].(string)
	token := paramsMap["token"].(string)
	m.logger.Debug("binding connection to MCP Manager, sessionID: %s, visionURL: %s", sessionID, visionURL)
	if !m.isInitialized {
		m.logger.Info("BindConnection: MCP Manager uninitialized, pre-initializing MCP servers")
		m.preInitializeServers()
	}

	// Check if XiaoZhiMCPClient needs restart
	if m.XiaoZhiMCPClient == nil {
		m.XiaoZhiMCPClient = NewXiaoZhiMCPClient(m.logger, conn, sessionID)
		m.XiaoZhiMCPClient.SetVisionURL(visionURL)
		m.XiaoZhiMCPClient.SetID(deviceID, clientID)
		m.XiaoZhiMCPClient.SetToken(token)

		if err := m.XiaoZhiMCPClient.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start XiaoZhi MCP client: %v", err)
		}
	} else {
		// Rebind connection rather than recreating
		m.XiaoZhiMCPClient.SetConnection(conn)
		m.XiaoZhiMCPClient.SetID(deviceID, clientID)
		m.XiaoZhiMCPClient.SetToken(token)
		if !m.XiaoZhiMCPClient.IsReady() {
			m.logger.Info("XiaoZhi MCP client not ready, restarting")
			if err := m.XiaoZhiMCPClient.Start(context.Background()); err != nil {
				return fmt.Errorf("failed to restart XiaoZhi MCP client: %v", err)
			}
		}
	}
	m.clients["xiaozhi"] = m.XiaoZhiMCPClient

	// Re-register tools (unregistered only)
	m.registerAllToolsIfNeeded()
	return nil
}

// Register tools on demand
func (m *Manager) registerAllToolsIfNeeded() {
	if m.funcHandler == nil {
		return
	}

	// Check registration to avoid duplicates
	if m.XiaoZhiMCPClient != nil && m.XiaoZhiMCPClient.IsReady() {
		tools := m.XiaoZhiMCPClient.GetAvailableTools()
		for _, tool := range tools {
			toolName := tool.Function.Name
			m.funcHandler.RegisterFunction(toolName, tool)
		}
		m.bRegisteredXiaoZhiMCP = true
	}

	// Register external MCP client tools
	for name, client := range m.clients {
		if name != "xiaozhi" && client.IsReady() {
			tools := client.GetAvailableTools()
			for _, tool := range tools {
				toolName := tool.Function.Name
				m.funcHandler.RegisterFunction(toolName, tool)
				if !m.isToolRegistered(toolName) {
					m.tools = append(m.tools, toolName)
					m.logger.Info("Registered external MCP tool: [%s] %s", toolName, tool.Function.Description)
				}
			}
		}
	}
}

// Helper methods
func (m *Manager) isToolRegistered(toolName string) bool {
	for _, tool := range m.tools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// Reset method
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset connection state while preserving reusable client struct
	m.conn = nil
	m.funcHandler = nil
	m.bRegisteredXiaoZhiMCP = false
	m.tools = make([]string, 0)

	// Reset connection rather than destroying client
	if m.XiaoZhiMCPClient != nil {
		m.XiaoZhiMCPClient.ResetConnection() // Reset connection
	}

	// Reset external MCP client connections
	/*
		for name, client := range m.clients {
			if name != "xiaozhi" {
				if resetter, ok := client.(interface{ ResetConnection() error }); ok {
					resetter.ResetConnection()
				}
			}
		}
	*/

	return nil
}

// Cleanup implements Provider Cleanup method
func (m *Manager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m.CleanupAll(ctx)
	return m.Reset()
}

// LoadConfig loads MCP service configuration
func (m *Manager) LoadConfig() map[string]interface{} {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		m.logger.Error(fmt.Sprintf("Error loading MCP config from %s: %v", m.configPath, err))
		return nil
	}

	var config struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		m.logger.Error(fmt.Sprintf("Error parsing MCP config: %v", err))
		return nil
	}

	return config.MCPServers
}

func (m *Manager) HandleXiaoZhiMCPMessage(msgMap map[string]interface{}) error {
	// Process Xiaozhi MCP message
	if m.XiaoZhiMCPClient == nil {
		return fmt.Errorf("XiaoZhiMCPClient is not initialized")
	}
	err := m.XiaoZhiMCPClient.HandleMCPMessage(msgMap)
	if err != nil {
		m.logger.Error(fmt.Sprintf("failed to process Xiaozhi MCP message: %v", err))
		return err
	}
	if m.XiaoZhiMCPClient.IsReady() && !m.bRegisteredXiaoZhiMCP {
		// Register Xiaozhi MCP tools
		m.registerTools(m.XiaoZhiMCPClient.GetAvailableTools())
		m.bRegisteredXiaoZhiMCP = true
	}
	return nil
}

// convertConfig converts map config to Config struct
func convertConfig(cfg map[string]interface{}) (*Config, error) {
	// Convert map to Config struct
	config := &Config{
		Enabled: true, // Enabled by default
	}

	// Server address
	if addr, ok := cfg["server_address"].(string); ok {
		config.ServerAddress = addr
	}

	// Server port
	if port, ok := cfg["server_port"].(float64); ok {
		config.ServerPort = int(port)
	}

	// Namespace
	if ns, ok := cfg["namespace"].(string); ok {
		config.Namespace = ns
	}

	// Node ID
	if nodeID, ok := cfg["node_id"].(string); ok {
		config.NodeID = nodeID
	}

	// Command-line execution
	if cmd, ok := cfg["command"].(string); ok {
		config.Command = cmd
	}

	// Command arguments
	if args, ok := cfg["args"].([]interface{}); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				config.Args = append(config.Args, argStr)
			}
		}
	}

	// Enabled flag
	if enabled, ok := cfg["enabled"].(bool); ok {
		config.Enabled = enabled
	}

	// Environment variables
	if env, ok := cfg["env"].(map[string]interface{}); ok {
		config.Env = make([]string, 0)
		for k, v := range env {
			if vStr, ok := v.(string); ok {
				config.Env = append(config.Env, fmt.Sprintf("%s=%s", k, vStr))
			}
		}
	}

	// SSE connection URL
	if url, ok := cfg["url"].(string); ok {
		config.URL = url
	}

	return config, nil
}

// registerTools registers tools
func (m *Manager) registerTools(tools []go_openai.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tool := range tools {
		toolName := tool.Function.Name

		// Check if tool is registered
		if m.isToolRegistered(toolName) {
			continue // Skip registered tools
		}

		m.tools = append(m.tools, toolName)
		if m.funcHandler != nil {
			if err := m.funcHandler.RegisterFunction(toolName, tool); err != nil {
				m.logger.Error(fmt.Sprintf("failed to register tool: %s, error: %v", toolName, err))
				continue
			}
			// m.logger.Info("Registered tool: [%s] %s", toolName, tool.Function.Description)
		}
	}
}

// IsMCPTool checks if name is MCP tool
func (m *Manager) IsMCPTool(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, tool := range m.tools {
		if tool == toolName {
			return true
		}
	}

	return false
}

// ExecuteTool executes tool invocation
func (m *Manager) ExecuteTool(
	ctx context.Context,
	toolName string,
	arguments map[string]interface{},
) (interface{}, error) {
	m.logger.Info(fmt.Sprintf("Executing tool %s with arguments: %v", toolName, arguments))

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		if client.HasTool(toolName) {
			return client.CallTool(ctx, toolName, arguments)
		}
	}

	clientNames := make([]string, 0, len(m.clients))
	for name := range m.clients {
		clientNames = append(clientNames, name)
	}

	return nil, fmt.Errorf("Tool %s not found in any MCP server， %v", toolName, clientNames)
}

// CleanupAll closes all MCP clients
func (m *Manager) CleanupAll(ctx context.Context) {
	m.mu.Lock()
	clients := make(map[string]MCPClient, len(m.clients))
	for name, client := range m.clients {
		clients[name] = client
	}
	m.mu.Unlock()

	for name, client := range clients {
		func() {
			// Set timeout context
			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				client.Stop()
				close(done)
			}()

			select {
			case <-done:
				m.logger.Info(fmt.Sprintf("MCP client closed: %s", name))
			case <-ctx.Done():
				m.logger.Error(fmt.Sprintf("Timeout closing MCP client %s", name))
			}
		}()

		m.mu.Lock()
		delete(m.clients, name)
		m.mu.Unlock()
	}
	m.isInitialized = false
}
