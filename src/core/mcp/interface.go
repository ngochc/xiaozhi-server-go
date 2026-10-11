package mcp

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// Tool represents MCP tool
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// MCPClient defines MCP client interface
type MCPClient interface {
	// Start starts MCP client
	Start(ctx context.Context) error

	// Stop stops MCP client
	Stop()

	// HasTool checks if tool with given name exists
	HasTool(name string) bool

	// GetAvailableTools gets all available tools
	GetAvailableTools() []openai.Tool

	// CallTool calls specified tool
	CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)

	// IsReady checks if client is initialized and ready
	IsReady() bool

	// ResetConnection resets connection state while preserving client struct
	ResetConnection() error
}

// Ensure Client implements MCPClient interface
var _ MCPClient = (*Client)(nil)
