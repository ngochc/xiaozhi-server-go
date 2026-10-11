package transport

import (
	"context"
	"net/http"
	"xiaozhi-server-go/src/core"
	"xiaozhi-server-go/src/core/mcp"
)

// Transport transport layer interface
type Transport interface {
	// Start transport service
	Start(ctx context.Context) error
	// Stop transport service
	Stop() error
	// Set connection handler factory
	SetConnectionHandler(handler ConnectionHandlerFactory)
	// Get active connection count
	GetActiveConnectionCount() (int, int)
	// Get transport type
	GetType() string
}

type Connection = core.Connection

// ConnectionHandler connection handler interface
type ConnectionHandler interface {
	// Handle connection
	Handle()
	// Close handler
	Close()
	// Get session ID
	GetSessionID() string
}

// ConnectionHandlerFactory connection handler factory interface
type ConnectionHandlerFactory interface {
	// Create connection handler
	CreateHandler(conn Connection, req *http.Request) ConnectionHandler
}

type MCPManagerHolder interface {
	GetMCPManager() *mcp.Manager
	SetMCPManager(*mcp.Manager)
}
