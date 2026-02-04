package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// Client represents a WebSocket client connection
type Client struct {
	// Unique identifier for this connection
	ID string

	// User information
	UserID   uint32
	TenantID uint32
	Username string

	// Actor information (for actor clients)
	IsActor bool
	RobotID string

	// WebSocket connection
	conn *websocket.Conn

	// Protocol type for this client
	protocolType protocol.ProtocolType

	// Send channel for outgoing messages
	send chan []byte

	// Context for this client
	ctx    context.Context
	cancel context.CancelFunc

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Last activity time for heartbeat tracking
	lastActivity time.Time

	// Manager reference
	manager *Manager

	// Ensure Close is only called once
	closeOnce sync.Once
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		ID:           uuid.New().String(),
		conn:         conn,
		send:         make(chan []byte, 256),
		ctx:          ctx,
		cancel:       cancel,
		lastActivity: time.Now(),
		manager:      manager,
	}
}

// SetUserInfo sets the user information for this client
func (c *Client) SetUserInfo(userID, tenantID uint32, username string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.UserID = userID
	c.TenantID = tenantID
	c.Username = username
}

// SetActorInfo sets the actor information for this client
func (c *Client) SetActorInfo(robotID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.IsActor = true
	c.RobotID = robotID
}

// SetProtocolType sets the protocol type for this client
func (c *Client) SetProtocolType(protocolType protocol.ProtocolType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.protocolType = protocolType
}

// GetProtocolType returns the protocol type for this client
func (c *Client) GetProtocolType() protocol.ProtocolType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.protocolType
}

// UpdateActivity updates the last activity time
func (c *Client) UpdateActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastActivity = time.Now()
}

// GetLastActivity returns the last activity time
func (c *Client) GetLastActivity() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastActivity
}

// SendMessage sends a message to the client
func (c *Client) SendMessage(data []byte) error {
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		// Channel is full, client is slow
		return ErrClientSlow
	}
}

// SendResponse sends a unified response to the client
func (c *Client) SendResponse(resp *protocol.UnifiedResponse, action string) error {
	adapter := protocol.NewAdapter()
	data, err := adapter.ConvertResponse(resp, c.GetProtocolType(), action)
	if err != nil {
		return err
	}

	return c.SendMessage(data)
}

// Close closes the client connection
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.send)
		err = c.conn.Close()
	})
	return err
}

// ReadPump pumps messages from the WebSocket connection to the manager
func (c *Client) ReadPump(readTimeout time.Duration, maxMessageSize int64) {
	defer func() {
		c.manager.Unregister(c)
		c.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	c.conn.SetReadLimit(maxMessageSize)

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		c.UpdateActivity()
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close
			}
			break
		}

		// Reset read deadline on any message received
		c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		c.UpdateActivity()

		// Handle the message through the manager
		c.manager.HandleMessage(c, message)
	}
}

// WritePump pumps messages from the send channel to the WebSocket connection
func (c *Client) WritePump(writeTimeout time.Duration, pingInterval time.Duration) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// Context returns the client context
func (c *Client) Context() context.Context {
	return c.ctx
}
