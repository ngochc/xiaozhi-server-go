package websocket

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/transport"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gorilla/websocket"
)

// WebSocketTransport WebSocket transport layer implementation
type WebSocketTransport struct {
	config            *configs.Config
	server            *http.Server
	logger            *utils.Logger
	connHandler       transport.ConnectionHandlerFactory
	activeConnections sync.Map
	upgrader          *websocket.Upgrader
}

// NewWebSocketTransport creates a new WebSocket transport layer
func NewWebSocketTransport(config *configs.Config, logger *utils.Logger) *WebSocketTransport {
	return &WebSocketTransport{
		config: config,
		logger: logger,
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from all origins
			},
		},
	}
}

// Start starts WebSocket transport layer
func (t *WebSocketTransport) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", t.config.Transport.WebSocket.IP, t.config.Transport.WebSocket.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleWebSocket)

	t.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	t.logger.Info("starting WebSocket transport layer at ws://%s", addr)

	// Listen for shutdown signal
	go func() {
		<-ctx.Done()
		t.Stop()
	}()

	if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start WebSocket transport layer: %v", err)
	}

	return nil
}

// Stop stops WebSocket transport layer
func (t *WebSocketTransport) Stop() error {
	if t.server != nil {
		t.logger.Info("stopping WebSocket transport layer...")

		// Close all active connections
		t.activeConnections.Range(func(key, value interface{}) bool {
			if handler, ok := value.(transport.ConnectionHandler); ok {
				handler.Close()
			}
			t.activeConnections.Delete(key)
			return true
		})

		return t.server.Close()
	}
	return nil
}

// SetConnectionHandler sets connection handler factory
func (t *WebSocketTransport) SetConnectionHandler(handler transport.ConnectionHandlerFactory) {
	t.connHandler = handler
}

// GetActiveConnectionCount gets active connection count
func (t *WebSocketTransport) GetActiveConnectionCount() (int, int) {
	count := 0
	t.activeConnections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count, count
}

// GetType gets transport type
func (t *WebSocketTransport) GetType() string {
	return "websocket"
}

// handleWebSocket handles WebSocket connection
func (t *WebSocketTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		t.logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	deviceID := r.Header.Get("Device-Id")
	clientID := r.Header.Get("Client-Id")
	if deviceID == "" {
		// Attempt to get from URL
		deviceID = r.URL.Query().Get("device-id")
		r.Header.Set("Device-Id", deviceID)
		t.logger.Info("attempting to get Device-Id from URL: %v", r.URL)
	}
	if clientID == "" {
		// Attempt to get from URL
		clientID = r.URL.Query().Get("client-id")
		r.Header.Set("Client-Id", clientID)
	}
	if clientID == "" {
		clientID = fmt.Sprintf("%p", conn)
	}
	t.logger.Info("[WebSocket] [connection request %s/%s]", deviceID, clientID)
	wsConn := NewWebSocketConnection(clientID, conn)

	if t.connHandler == nil {
		t.logger.Error("connection handler factory not set")
		conn.Close()
		return
	}

	handler := t.connHandler.CreateHandler(wsConn, r)
	if handler == nil {
		t.logger.Error("failed to create connection handler")
		conn.Close()
		return
	}

	t.activeConnections.Store(clientID, handler)
	t.logger.Info("[WebSocket] [connection established %s] resources allocated", clientID)

	// Start connection handling and cleanup resources on completion
	go func() {
		defer func() {
			// Cleanup on connection end
			t.activeConnections.Delete(clientID)
			handler.Close()
		}()

		handler.Handle()
	}()
}
