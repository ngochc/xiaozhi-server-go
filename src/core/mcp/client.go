package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
	"xiaozhi-server-go/src/core/types"
	"xiaozhi-server-go/src/core/utils"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sashabaranov/go-openai"
)

// Config defines MCP client configuration
type Config struct {
	Enabled       bool     `yaml:"enabled"`
	ServerAddress string   `yaml:"server_address"`
	ServerPort    int      `yaml:"server_port"`
	Namespace     string   `yaml:"namespace"`
	NodeID        string   `yaml:"node_id"`
	ResourceTypes []string `yaml:"resource_types"`
	Command       string   `yaml:"command,omitempty"` // Command-line execution
	Args          []string `yaml:"args,omitempty"`    // Command arguments
	Env           []string `yaml:"env,omitempty"`     // Environment variables
	URL           string   `yaml:"url,omitempty"`     // SSE connection URL
}

// Client encapsulates MCP client functionality
type Client struct {
	client         *mcpclient.Client
	stdioClient    *mcpclient.Client
	config         *Config
	name           string
	tools          []Tool
	ready          bool
	mu             sync.RWMutex
	useStdioClient bool
	logger         *utils.Logger
}

// NewClient creates a new MCP client instance
func NewClient(config *Config, logger *utils.Logger) (*Client, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("MCP client is disabled in config")
	}

	c := &Client{
		config: config,
		tools:  make([]Tool, 0),
		ready:  false,
		logger: logger,
	}

	// Select client type based on config
	if config.Command != "" {
		// Connect via command line
		stdioClient, err := mcpclient.NewStdioMCPClient(
			config.Command,
			config.Env,
			config.Args...,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdio MCP client: %w", err)
		}
		c.stdioClient = stdioClient
		c.useStdioClient = true
	} else {
		fmt.Println("Unsupported MCP client type, only stdio client is supported")
	}

	return c, nil
}

// Start starts MCP client and listens for updates
func (c *Client) Start(ctx context.Context) error {
	if c.useStdioClient {
		// c.logger.Info("Starting MCP stdio client with command: %s", c.config.Command)

		// Create initialization request
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "zhi-server",
			Version: "1.0.0",
		}

		// Set timeout context
		initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Initialize client
		initResult, err := c.stdioClient.Initialize(initCtx, initRequest)
		if err != nil {
			return fmt.Errorf("failed to initialize stdio MCP client: %w", err)
		}
		c.name = initResult.ServerInfo.Name
		c.logger.Info("Initialized server: %s %s with conmmand: %s",
			initResult.ServerInfo.Name,
			initResult.ServerInfo.Version,
			c.config.Command)

		// Get tools list
		err = c.fetchTools(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch tools: %w", err)
		}
	}

	c.mu.Lock()
	c.ready = true
	c.mu.Unlock()

	return nil
}

// fetchTools fetches list of available tools
func (c *Client) fetchTools(ctx context.Context) error {
	if c.useStdioClient {
		// Fetch tools list via protocol
		toolsRequest := mcp.ListToolsRequest{}
		tools, err := c.stdioClient.ListTools(ctx, toolsRequest)
		if err != nil {
			return fmt.Errorf("failed to list tools: %w", err)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		// Clear current tools list
		c.tools = make([]Tool, 0, len(tools.Tools))

		// Add retrieved tools
		toolNames := ""
		for _, tool := range tools.Tools {
			required := tool.InputSchema.Required
			if required == nil {
				required = make([]string, 0)
			}
			c.tools = append(c.tools, Tool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: ToolInputSchema{
					Type:       tool.InputSchema.Type,
					Properties: tool.InputSchema.Properties,
					Required:   required,
				},
			})
			toolNames += fmt.Sprintf("%s, ", tool.Name)
			// log.Printf("Added tool: %s - %s %v; %v; %v", tool.Name, tool.Description, tool.InputSchema, tool.RawInputSchema, tool.Annotations)
		}
		c.logger.Info("Fetching %s available tools %s", c.name, toolNames)
		return nil
	} else {
		// Legacy implementation remains unchanged
		// Tool information can be retrieved via resource types
		return nil
	}
}

// Stop stops MCP client
func (c *Client) Stop() {
	if c.useStdioClient {
		if c.stdioClient != nil {
			c.logger.Info("Stopping MCP stdio client")
			c.stdioClient.Close()
		}
	} else {
		if c.client != nil {
			c.logger.Info("Stopping MCP client")
			c.client.Close()
		}
	}

	c.mu.Lock()
	c.ready = false
	c.mu.Unlock()
}

// HasTool checks if tool with name exists
func (c *Client) HasTool(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Strip mcp_ prefix if present
	if len(name) > 4 && name[:4] == "mcp_" {
		name = name[4:]
	}

	for _, tool := range c.tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// GetAvailableTools gets all available tools
func (c *Client) GetAvailableTools() []openai.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]openai.Tool, 0, len(c.tools))
	for _, tool := range c.tools {
		openaiTool := openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        fmt.Sprintf("mcp_%s", tool.Name),
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

// CallTool calls specified tool
func (c *Client) CallTool(
	ctx context.Context,
	name string,
	args map[string]any,
) (interface{}, error) {
	// Strip mcp_ prefix if present
	if len(name) > 4 && name[:4] == "mcp_" {
		name = name[4:]
	}
	// Check if tool exists
	if !c.HasTool(name) {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	if c.useStdioClient {
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = name
		callRequest.Params.Arguments = args

		result, err := c.stdioClient.CallTool(ctx, callRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
		}

		// Process result
		if result == nil || len(result.Content) == 0 {
			return nil, nil
		}

		// Return first content item or entire list
		if len(result.Content) == 1 {
			// If text content, return text directly
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				return textContent.Text, nil
			}
			ret := types.ActionResponse{
				Action: types.ActionTypeReqLLM,
				Result: result.Content[0],
			}
			return ret, nil
		}

		// Handle multiple content items
		processedContent := make([]interface{}, 0, len(result.Content))
		for _, content := range result.Content {
			if textContent, ok := content.(mcp.TextContent); ok {
				processedContent = append(processedContent, textContent.Text)
			} else {
				processedContent = append(processedContent, content)
			}
		}
		ret := types.ActionResponse{
			Action: types.ActionTypeReqLLM,
			Result: processedContent,
		}
		return ret, nil
	}

	// Raw network client does not support calling tools directly
	return nil, fmt.Errorf("tool calling not implemented for network client")
}

// IsReady checks if client is initialized and ready
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// ResetConnection resets connection state
func (c *Client) ResetConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Preserve tool info, reset connection state only
	c.ready = false

	// Gracefully close active connection if any
	if c.useStdioClient && c.stdioClient != nil {
		// Mark not ready without full teardown
		// Re-establishes connection on next Start
	}

	return nil
}
