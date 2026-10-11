package websocket

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConnection WebSocket connection adapter
type WebSocketConnection struct {
	id         string
	conn       *websocket.Conn
	closed     int32
	lastActive int64
	mu         sync.Mutex
}

// NewWebSocketConnection creates a new WebSocket connection adapter
func NewWebSocketConnection(id string, conn *websocket.Conn) *WebSocketConnection {
	return &WebSocketConnection{
		id:         id,
		conn:       conn,
		closed:     0,
		lastActive: time.Now().Unix(),
	}
}

// WriteMessage sends message
func (c *WebSocketConnection) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.closed) == 1 {
		return fmt.Errorf("connection is closed")
	}

	atomic.StoreInt64(&c.lastActive, time.Now().Unix())
	return c.conn.WriteMessage(messageType, data)
}

// ReadMessage reads message
func (c *WebSocketConnection) ReadMessage(stopChan <-chan struct{}) (int, []byte, error) {
	messageType, data, err := c.conn.ReadMessage()
	if err == nil {
		atomic.StoreInt64(&c.lastActive, time.Now().Unix())
	}
	return messageType, data, err
}

// Close closes connection
func (c *WebSocketConnection) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return c.conn.Close()
	}
	return nil
}

// GetID gets connection ID
func (c *WebSocketConnection) GetID() string {
	return c.id
}

// GetType gets connection type
func (c *WebSocketConnection) GetType() string {
	return "websocket"
}

// IsClosed checks if connection is closed
func (c *WebSocketConnection) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// GetLastActiveTime gets last active time
func (c *WebSocketConnection) GetLastActiveTime() time.Time {
	return time.Unix(atomic.LoadInt64(&c.lastActive), 0)
}

// IsStale checks if connection is expired
func (c *WebSocketConnection) IsStale(timeout time.Duration) bool {
	return time.Since(c.GetLastActiveTime()) > timeout
}
