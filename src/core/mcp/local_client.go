package mcp

import (
	"context"
	"fmt"
	"sync"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"

	"github.com/sashabaranov/go-openai"
)

type HandlerFunc func(ctx context.Context, args map[string]interface{}) (interface{}, error)

type LocalClient struct {
	tools   []Tool
	mu      sync.RWMutex
	ctx     context.Context
	logger  *utils.Logger
	handler map[string]HandlerFunc
	cfg     *configs.Config
}

func NewLocalClient(logger *utils.Logger, cfg *configs.Config) (*LocalClient, error) {
	c := &LocalClient{
		tools:   make([]Tool, 0),
		handler: make(map[string]HandlerFunc),
		mu:      sync.RWMutex{},
		logger:  logger,
		cfg:     cfg,
	}
	return c, nil
}

func (c *LocalClient) RegisterTools() {
	if c.cfg == nil {
		c.logger.Error("RegisterTools: config is nil")
		return
	}
	// Onboarding assistant must handle device takeover, agent creation and config independently of legacy LocalMCPFun.
	if err := c.AddToolSwitchAgent(); err != nil {
		c.logger.Debug("RegisterTools: switch_agent: %v", err)
	}
	if err := c.AddToolListAgents(); err != nil {
		c.logger.Debug("RegisterTools: list_agents: %v", err)
	}
	if err := c.AddToolCreateAgent(); err != nil {
		c.logger.Debug("RegisterTools: create_agent: %v", err)
	}
	if err := c.AddToolUpdateAgent(); err != nil {
		c.logger.Debug("RegisterTools: update_agent: %v", err)
	}

	if c.cfg.LocalMCPFun == nil {
		c.logger.Warn("RegisterTools: LocalMCPFun is nil; only onboarding tools are available")
		return
	}

	funcs := c.cfg.LocalMCPFun
	if len(funcs) == 0 {
		c.logger.Info("RegisterTools: LocalMCPFun is empty")
		return
	}

	for _, localFunc := range funcs {
		if localFunc.Name == "exit" && localFunc.Enabled {
			c.AddToolExit()
			c.logger.Info("RegisterTools: exit tool registered")
		} else if localFunc.Name == "time" && localFunc.Enabled {
			c.AddToolTime()
			c.logger.Info("RegisterTools: time tool registered")
		} else if localFunc.Name == "change_voice" && localFunc.Enabled {
			c.AddToolChangeVoice()
			c.logger.Info("RegisterTools: change_voice tool registered")
		} else if localFunc.Name == "change_role" && localFunc.Enabled {
			c.AddToolChangeRole()
			c.logger.Info("RegisterTools: change_role tool registered")
		} else if localFunc.Name == "play_music" && localFunc.Enabled {
			c.AddToolPlayMusic()
			c.logger.Info("RegisterTools: play_music tool registered")
		} else if localFunc.Name == "switch_agent" && localFunc.Enabled {
			c.AddToolSwitchAgent()
			c.logger.Info("RegisterTools: switch_agent tool registered")
		} else {
			if localFunc.Enabled {
				c.logger.Warn("RegisterTools: unknown function name %s", localFunc.Name)
			}
		}
	}
}

// Start starts local MCP client
func (c *LocalClient) Start(ctx context.Context) error {
	c.ctx = ctx
	c.RegisterTools()
	c.logger.Info("Local MCP client started")
	return nil
}

// Stop stops local MCP client
func (c *LocalClient) Stop() {
	// Local client requires no teardown, returns immediately
}

// HasTool checks if local client has tool with name
func (c *LocalClient) HasTool(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Strip local_ prefix if present
	if len(name) > 6 && name[:6] == "local_" {
		name = name[6:]
	}
	for _, tool := range c.tools {
		if tool.Name == name {
			return true
		}
	}

	return false
}

// GetAvailableTools gets all available tools on local client
func (c *LocalClient) GetAvailableTools() []openai.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]openai.Tool, 0, len(c.tools))
	for _, tool := range c.tools {
		openaiTool := openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        fmt.Sprintf("local_%s", tool.Name),
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type":       tool.InputSchema.Type,
					"properties": tool.InputSchema.Properties,
					"required":   tool.InputSchema.Required,
				},
			},
		}
		result = append(result, openaiTool)
	}
	return result
}

// CallTool calls specified tool on local client
func (c *LocalClient) CallTool(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (interface{}, error) {
	// Check if tool exists
	if !c.HasTool(name) {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	// Strip local_ prefix if present
	if len(name) > 6 && name[:6] == "local_" {
		name = name[6:]
	}
	var handler HandlerFunc
	c.mu.RLock()
	if h, ok := c.handler[name]; ok {
		handler = h
		c.mu.RUnlock()
	} else {
		c.mu.RUnlock()
		return nil, fmt.Errorf("handler for tool %s not found", name)
	}

	return handler(ctx, args)
}

// IsReady checks if local client is ready
func (c *LocalClient) IsReady() bool {
	// Local client is always ready
	return true
}

// ResetConnection resets local client connection state
func (c *LocalClient) ResetConnection() error {
	// Local client has no connection state, returns nil
	return nil
}

func (c *LocalClient) AddTool(
	name string,
	description string,
	input ToolInputSchema,
	handler HandlerFunc,
) error {
	if c.HasTool(name) {
		return fmt.Errorf("tool %s already exists", name)
	}

	tool := Tool{
		Name:        name,
		Description: description,
		InputSchema: input,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = append(c.tools, tool)
	c.handler[name] = handler
	return nil
}
