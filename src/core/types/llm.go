package types

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// ToolType represents the type of tool operation.
type ToolType int

const (
	ToolNone            ToolType = iota + 1 // 1
	ToolWait                                // 2
	ToolChangeSysPrompt                     // 3
	ToolSystemCtl                           // 4
	ToolIotCtl                              // 5
	ToolMcpClient                           // 6
)

var ToolTypeMessages = map[ToolType]string{
	ToolNone:            "no further action after tool call",
	ToolWait:            "call tool and wait for function return",
	ToolChangeSysPrompt: "modify system prompt to change role persona or duties",
	ToolSystemCtl:       "system control affecting normal dialogue flow like exit or playing music, requires conn param",
	ToolIotCtl:          "IoT device control, requires conn param",
	ToolMcpClient:       "MCP client",
}

// Action represents the type of action.
type Action int

const (
	ActionTypeError       Action = -1
	ActionTypeNotFound    Action = 0
	ActionTypeNone        Action = 1
	ActionTypeResponse    Action = 2
	ActionTypeReqLLM      Action = 3
	ActionTypeCallHandler Action = 4
)

var ActionDesc = map[Action]string{
	ActionTypeError:    "error",
	ActionTypeNotFound: "function not found",
	ActionTypeNone:     "none",
	ActionTypeResponse: "respond",
	ActionTypeReqLLM:   "call_function_then_request_llm",
}

// ActionResponse holds the result of an action.
type ActionResponse struct {
	Action   Action      // Action type
	Result   interface{} // Action result
	Response interface{} // Direct response content
}

type ActionResponseCall struct {
	FuncName string      // Function name
	Args     interface{} // Function arguments
}

// Message dialogue message struct
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

func (m *Message) Print() {
	// Convert to JSON string
	jsonStr, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Println("json marshal error:", err)
		return
	}
	// fmt.Println("Message:")
	fmt.Println(string(jsonStr))
}

// ToolCall tool call struct
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
	Index    int          `json:"index"`
}

// FunctionCall function call result
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Response LLM response struct
type Response struct {
	Content              string     `json:"content,omitempty"`
	ReasonContent        string     `json:"reasoning_content,omitempty"`
	UpdateConversationID string     `json:"update_conversation_id,omitempty"`
	ToolCalls            []ToolCall `json:"tool_calls,omitempty"`
	StopReason           string     `json:"stop_reason,omitempty"`
	Error                string     `json:"error,omitempty"`
}

// Provider base provider interface
type Provider interface {
	Initialize() error
	Cleanup() error
}

type FunctionRegistryInterface interface {
	RegisterFunction(name string, function openai.Tool) error
	GetFunction(name string) (openai.Tool, error)
	GetAllFunctions() []openai.Tool
	UnregisterFunction(name string) error
	UnregisterAllFunctions() error
	FunctionExists(name string) bool
}

// LLMProvider Large Language Model provider interface
type LLMProvider interface {
	Provider
	Response(ctx context.Context, sessionID string, messages []Message) (<-chan string, error)
	ResponseWithFunctions(
		ctx context.Context,
		sessionID string,
		messages []Message,
		tools []openai.Tool,
	) (<-chan Response, error)
	GetSessionID() string                       // Get current session ID
	SetIdentityFlag(idType string, flag string) // Set identity flag
}
