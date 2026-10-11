package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/types"
	"xiaozhi-server-go/src/core/utils"

	"github.com/sashabaranov/go-openai"
)

// MCP message ID constants
const (
	mcpInitializeID = 1 // Initialize message ID
	mcpToolsListID  = 2 // Tools list request ID
	mcpToolCallID   = 3 // Tool call request ID

	msgTypeText = 1 // Text message type
)

// XiaoZhiMCPClient MCP client
type XiaoZhiMCPClient struct {
	logger     *utils.Logger
	conn       Conn
	sessionID  string // Session ID identifying connection
	tools      []Tool
	ready      bool
	mu         sync.RWMutex
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Handler for tool call responses
	callResults     map[int]chan interface{}
	callResultsLock sync.Mutex
	nextID          int
	visionURL       string // Vision service URL
	deviceID        string // Device ID identifying device
	clientID        string // Client ID identifying client
	token           string // Access token
	// Tool name mapping: sanitized name -> original name
	toolNameMap map[string]string
}

// NewXiaoZhiMCPClient creates new MCP client
func NewXiaoZhiMCPClient(logger *utils.Logger, conn Conn, sessionID string) *XiaoZhiMCPClient {
	return &XiaoZhiMCPClient{
		logger:      logger,
		conn:        conn,
		sessionID:   sessionID,
		tools:       make([]Tool, 0),
		ready:       false,
		callResults: make(map[int]chan interface{}),
		nextID:      1,
		toolNameMap: make(map[string]string),
	}
}

// SetConnection sets new connection
func (c *XiaoZhiMCPClient) SetConnection(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

func (c *XiaoZhiMCPClient) SetID(deviceID string, clientID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deviceID = deviceID
	c.clientID = clientID // Use clientID as session ID
}

func (c *XiaoZhiMCPClient) SetToken(token string) {
	auth := auth.NewAuthToken(token)
	visionToken, err := auth.GenerateToken(c.deviceID)
	if err != nil {
		c.logger.Error(fmt.Sprintf("failed to generate Vision Token: %v", err))
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = visionToken
}

func (c *XiaoZhiMCPClient) SetVisionURL(visionURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.visionURL = visionURL
}

// ResetConnection resets connection state
func (c *XiaoZhiMCPClient) ResetConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn = nil
	c.ready = false
	c.sessionID = "" // Clear session ID
	c.tools = make([]Tool, 0)
	c.callResults = make(map[int]chan interface{})
	c.clientID = "" // Clear client ID
	c.deviceID = "" // Clear device ID
	c.token = ""    // Clear access token

	return nil
}

// Start starts MCP client
func (c *XiaoZhiMCPClient) Start(ctx context.Context) error {
	c.mu.Lock()
	c.ctx, c.cancelFunc = context.WithCancel(ctx)
	c.mu.Unlock()

	// Send initialization message
	return c.SendMCPInitializeMessage()
}

// Stop stops MCP client
func (c *XiaoZhiMCPClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	// Cleanup resources
	c.ready = false

	// Cancel pending tool calls
	c.callResultsLock.Lock()
	defer c.callResultsLock.Unlock()

	for id, ch := range c.callResults {
		close(ch)
		delete(c.callResults, id)
	}
}

// HasTool checks if tool with name exists (supports sanitized name)
func (c *XiaoZhiMCPClient) HasTool(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check original name first
	for _, tool := range c.tools {
		if tool.Name == name {
			return true
		}
	}

	// Check if sanitized name
	if _, exists := c.toolNameMap[name]; exists {
		return true
	}

	return false
}

func sanitizeToolName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

// GetAvailableTools gets all available tools
func (c *XiaoZhiMCPClient) GetAvailableTools() []openai.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]openai.Tool, 0, len(c.tools))
	for _, tool := range c.tools {
		result = append(result, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        sanitizeToolName(tool.Name),
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type":       tool.InputSchema.Type,
					"properties": tool.InputSchema.Properties,
					"required":   tool.InputSchema.Required,
				},
			},
		})
	}
	return result
}

// CallTool calls specified tool
func (c *XiaoZhiMCPClient) CallTool(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error(fmt.Sprintf("panic calling tool %s: %v", name, r))
		}
	}()
	if !c.IsReady() {
		return nil, fmt.Errorf("MCP client not ready")
	}

	// Get original tool name
	originalName := name
	if mappedName, exists := c.toolNameMap[name]; exists {
		originalName = mappedName
	} else if !c.HasTool(name) {
		return nil, fmt.Errorf("tool %s does not exist", name)
	}

	// Get next ID and create result channel
	c.callResultsLock.Lock()
	id := c.nextID
	c.nextID++
	resultCh := make(chan interface{}, 1)
	c.callResults[id] = resultCh
	c.callResultsLock.Unlock()

	// Construct tool call request
	mcpMessage := map[string]interface{}{
		"type":       "mcp",
		"session_id": c.sessionID, // Use connection session_id
		"payload": map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      originalName,
				"arguments": args,
			},
		},
	}

	data, err := json.Marshal(mcpMessage)
	if err != nil {
		// Cleanup resources
		c.callResultsLock.Lock()
		delete(c.callResults, id)
		c.callResultsLock.Unlock()
		return nil, fmt.Errorf("failed to serialize MCP tool call request: %v", err)
	}

	c.logger.Info(fmt.Sprintf("sending client MCP tool call request: %s, params: %s", originalName, string(data)))
	if c.conn == nil {
		// Cleanup resources
		c.callResultsLock.Lock()
		delete(c.callResults, id)
		c.callResultsLock.Unlock()
		return nil, fmt.Errorf("MCP client not connected")
	}

	err = c.conn.WriteMessage(msgTypeText, data)
	if err != nil {
		// Cleanup resources
		c.callResultsLock.Lock()
		delete(c.callResults, id)
		c.callResultsLock.Unlock()
		return nil, err
	}

	// Wait for response or timeout
	select {
	case result := <-resultCh:
		if err, ok := result.(error); ok {
			return nil, err
		}
		c.logger.Info(fmt.Sprintf("client MCP tool call %s succeeded, result: %v", originalName, result))
		//  map[content:[map[text:{"audio_speaker":{"volume":10},"screen":{},"network":{"type":"wifi","ssid":"zgcinnotown","signal":"weak"}} type:text]] isError:false]
		// Extract text content
		if resultMap, ok := result.(map[string]interface{}); ok {
			// Check if isError is true
			if isError, ok := resultMap["isError"].(bool); ok && isError {
				if errorMsg, ok := resultMap["error"].(string); ok {
					return nil, fmt.Errorf("tool call error: %s", errorMsg)
				}
				return nil, fmt.Errorf("tool call returned error without details")
			}
			// Check if content field exists and is non-empty array
			if content, ok := resultMap["content"].([]interface{}); ok && len(content) > 0 {
				if textMap, ok := content[0].(map[string]interface{}); ok {
					if text, ok := textMap["text"].(string); ok {
						if strings.Contains(originalName, "self.camera.take_photo") {
							ret := types.ActionResponse{
								Action: types.ActionTypeCallHandler,
								Result: types.ActionResponseCall{
									FuncName: "mcp_handler_take_photo",
									Args:     text,
								},
							}
							return ret, nil
						}
						c.logger.Info(fmt.Sprintf("tool call returned text: %s", text))
						ret := types.ActionResponse{
							Action: types.ActionTypeReqLLM,
							Result: text,
						}
						return ret, nil
					}
				}
			}
		}
		return result, nil
	case <-ctx.Done():
		// Context cancelled or timed out
		c.callResultsLock.Lock()
		delete(c.callResults, id)
		c.callResultsLock.Unlock()
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		// Request timed out
		c.callResultsLock.Lock()
		delete(c.callResults, id)
		c.callResultsLock.Unlock()
		return nil, fmt.Errorf("tool call request timed out")
	}
}

// IsReady checks if client is initialized and ready
func (c *XiaoZhiMCPClient) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// SendMCPInitializeMessage sends MCP initialize message
func (c *XiaoZhiMCPClient) SendMCPInitializeMessage() error {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error(fmt.Sprintf("panic sending MCP initialize message: %v", r))
		}
	}()

	// Construct MCP initialize message
	mcpMessage := map[string]interface{}{
		"type":       "mcp",
		"session_id": c.sessionID,
		"payload": map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      mcpInitializeID,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"roots": map[string]interface{}{
						"listChanged": true,
					},
					"sampling": map[string]interface{}{},
					"vision": map[string]interface{}{
						"url":   c.visionURL,
						"token": c.token,
					},
				},
				"clientInfo": map[string]interface{}{
					"name":    "XiaozhiClient",
					"version": "1.0.0",
				},
			},
		},
	}

	data, err := json.Marshal(mcpMessage)
	if err != nil {
		return fmt.Errorf("failed to serialize MCP initialize message: %v", err)
	}

	c.logger.Info("[MCP] [Initialize] sending initialize message")
	if c.conn == nil {
		return fmt.Errorf("MCP client not connected")
	}
	return c.conn.WriteMessage(msgTypeText, data)
}

// SendMCPToolsListRequest sends MCP tools/list request
func (c *XiaoZhiMCPClient) SendMCPToolsListRequest() error {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error(fmt.Sprintf("panic sending MCP tools/list request: %v", r))
		}
	}()

	// Construct MCP tools/list request
	mcpMessage := map[string]interface{}{
		"type":       "mcp",
		"session_id": c.sessionID,
		"payload": map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      mcpToolsListID, // Use new ID
			"method":  "tools/list",
		},
	}

	data, err := json.Marshal(mcpMessage)
	if err != nil {
		return fmt.Errorf("failed to serialize MCP tools/list request: %v", err)
	}

	c.logger.Debug("sending MCP tools/list request")
	if c.conn == nil {
		return fmt.Errorf("MCP client not connected")
	}
	return c.conn.WriteMessage(msgTypeText, data)
}

// SendMCPToolsListContinueRequest sends MCP tools/list request with cursor
func (c *XiaoZhiMCPClient) SendMCPToolsListContinueRequest(cursor string) error {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error(fmt.Sprintf("panic sending MCP tools/list cursor request: %v", r))
		}
	}()

	// Construct MCP tools/list request
	mcpMessage := map[string]interface{}{
		"type":       "mcp",
		"session_id": c.sessionID,
		"payload": map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      mcpToolsListID, // Use same ID
			"method":  "tools/list",
			"params": map[string]interface{}{
				"cursor": cursor,
			},
		},
	}

	data, err := json.Marshal(mcpMessage)
	if err != nil {
		return fmt.Errorf("failed to serialize MCP tools/list request: %v", err)
	}

	c.logger.Info(fmt.Sprintf("sending MCP tools/list request with cursor: %s", cursor))
	if c.conn == nil {
		return fmt.Errorf("MCP client not connected")
	}
	return c.conn.WriteMessage(msgTypeText, data)
}

// HandleMCPMessage processes MCP message
func (c *XiaoZhiMCPClient) HandleMCPMessage(msgMap map[string]interface{}) error {
	// c.logger.Info("process MCP message: " + fmt.Sprintf("%v", msgMap))
	// Get payload
	payload, ok := msgMap["payload"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("MCP message missing payload field")
	}

	// Check for response field
	result, hasResult := payload["result"]
	if hasResult {
		// Get ID to match request response
		id, _ := payload["id"].(float64)
		idInt := int(id)

		// Check if tool call response
		c.callResultsLock.Lock()
		if resultCh, ok := c.callResults[idInt]; ok {
			resultCh <- result
			delete(c.callResults, idInt)
			c.callResultsLock.Unlock()
			return nil
		}
		c.callResultsLock.Unlock()

		if id == mcpInitializeID { // Initialization response
			c.logger.Debug("received MCP initialize response")

			// Parse server info
			if serverInfo, ok := result.(map[string]interface{})["serverInfo"].(map[string]interface{}); ok {
				name := serverInfo["name"]
				version := serverInfo["version"]
				c.logger.Info(fmt.Sprintf("[MCP] [Server Info %v/%v]", name, version))
			}

			// Request tools list after initialization
			return c.SendMCPToolsListRequest()
		} else if id == mcpToolsListID { // tools/list response
			c.logger.Debug("received MCP tools/list response")

			// Parse tools list
			toolNames := ""
			if toolsData, ok := result.(map[string]interface{}); ok {
				tools, ok := toolsData["tools"].([]interface{})
				if !ok {
					return fmt.Errorf("invalid tools list format")
				}

				c.logger.Debug(fmt.Sprintf("tools supported by client device: %d", len(tools)))

				// Parse tools and add to list
				c.mu.Lock()
				for i, tool := range tools {
					toolMap, ok := tool.(map[string]interface{})
					if !ok {
						continue
					}

					// Construct Tool struct and add to list
					name, _ := toolMap["name"].(string)
					desc, _ := toolMap["description"].(string)

					inputSchema := ToolInputSchema{
						Type: "object",
					}

					if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
						if schemaType, ok := schema["type"].(string); ok {
							inputSchema.Type = schemaType
						}

						if properties, ok := schema["properties"].(map[string]interface{}); ok {
							inputSchema.Properties = properties
						}

						if required, ok := schema["required"].([]interface{}); ok {
							inputSchema.Required = make([]string, 0, len(required))
							for _, r := range required {
								if s, ok := r.(string); ok {
									inputSchema.Required = append(inputSchema.Required, s)
								}
							}
						} else {
							inputSchema.Required = make([]string, 0) // Ensure empty slice rather than nil
						}
					}

					newTool := Tool{
						Name:        name,
						Description: desc,
						InputSchema: inputSchema,
					}

					c.tools = append(c.tools, newTool)
					// Map tool names
					sanitizedName := sanitizeToolName(name)
					c.toolNameMap[sanitizedName] = name
					c.logger.Debug(fmt.Sprintf("client tool #%d: %v", i+1, name))
					toolNames += fmt.Sprintf("%s ", name)
				}
				c.logger.Info(fmt.Sprintf("[MCP] [Tool List] %s", toolNames))

				// Check if pagination cursor exists
				if nextCursor, ok := toolsData["nextCursor"].(string); ok && nextCursor != "" {
					// Send request with cursor if more tools exist
					c.logger.Info(fmt.Sprintf("more tools available, nextCursor: %s", nextCursor))
					c.mu.Unlock()
					return c.SendMCPToolsListContinueRequest(nextCursor)
				} else {
					// All tools retrieved, set ready flag
					c.ready = true
				}
				c.mu.Unlock()
			}
		}
	} else if method, hasMethod := payload["method"].(string); hasMethod {
		// Handle client-initiated requests
		c.logger.Info(fmt.Sprintf("received MCP client request: %s", method))
		// TODO: Implement handling of client requests
	} else if errorData, hasError := payload["error"].(map[string]interface{}); hasError {
		// Handle error responses
		errorMsg, _ := errorData["message"].(string)
		c.logger.Error(fmt.Sprintf("received MCP error response: %v", errorMsg))

		// Check if tool call response
		if id, ok := payload["id"].(float64); ok {
			idInt := int(id)

			c.callResultsLock.Lock()
			if resultCh, ok := c.callResults[idInt]; ok {
				resultCh <- fmt.Errorf("MCP error: %s", errorMsg)
				delete(c.callResults, idInt)
			}
			c.callResultsLock.Unlock()
		}
	}

	return nil
}
