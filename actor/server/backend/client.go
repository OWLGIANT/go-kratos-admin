package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"actor/third/log"
)

const (
	// Default configuration values
	defaultWriteWait      = 10 * time.Second
	defaultPongWait       = 60 * time.Second
	defaultPingPeriod     = 30 * time.Second
	defaultReconnectDelay = 5 * time.Second
	defaultMaxReconnect   = 0 // 0 means unlimited
)

// ClientConfig holds configuration for the backend client
type ClientConfig struct {
	URL            string        // WebSocket URL (e.g., ws://backend:7790/ws)
	Token          string        // JWT token for authentication
	RobotID        string        // Robot identifier
	Exchange       string        // Exchange name
	Version        string        // Actor version
	TenantID       uint32        // Tenant ID
	WriteWait      time.Duration // Time allowed to write a message
	PongWait       time.Duration // Time allowed to read the next pong message
	PingPeriod     time.Duration // Send pings to peer with this period
	ReconnectDelay time.Duration // Delay between reconnection attempts
	MaxReconnect   int           // Maximum reconnection attempts (0 = unlimited)
}

// Client manages the WebSocket connection to backend
type Client struct {
	config   ClientConfig
	conn     *websocket.Conn
	handler  CommandHandler
	logger   log.Logger
	sendChan chan []byte

	ctx    context.Context
	cancel context.CancelFunc

	mu          sync.RWMutex
	connected   bool
	reconnCount int

	// Callbacks
	onConnect    func()
	onDisconnect func(error)
}

// NewClient creates a new backend client
func NewClient(config ClientConfig, handler CommandHandler, logger log.Logger) *Client {
	// Set defaults
	if config.WriteWait == 0 {
		config.WriteWait = defaultWriteWait
	}
	if config.PongWait == 0 {
		config.PongWait = defaultPongWait
	}
	if config.PingPeriod == 0 {
		config.PingPeriod = defaultPingPeriod
	}
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = defaultReconnectDelay
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:   config,
		handler:  handler,
		logger:   logger,
		sendChan: make(chan []byte, 256),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// OnConnect sets the callback for successful connection
func (c *Client) OnConnect(fn func()) {
	c.onConnect = fn
}

// OnDisconnect sets the callback for disconnection
func (c *Client) OnDisconnect(fn func(error)) {
	c.onDisconnect = fn
}

// Connect establishes connection to backend
func (c *Client) Connect() error {
	return c.connect()
}

// connect performs the actual connection
func (c *Client) connect() error {
	// Build URL with token
	url := c.config.URL
	if c.config.Token != "" {
		url = fmt.Sprintf("%s?token=%s", url, c.config.Token)
	}

	c.logger.Infof("Connecting to backend: %s", c.config.URL)

	// Add actor identification headers
	header := make(map[string][]string)
	header["X-Actor-Client"] = []string{"true"}
	header["X-Actor-Robot-ID"] = []string{c.config.RobotID}
	header["X-Actor-Tenant-ID"] = []string{fmt.Sprintf("%d", c.config.TenantID)}
	if c.config.Token != "" {
		header["X-Actor-Token"] = []string{c.config.Token}
	}

	// Dial WebSocket with custom headers
	conn, _, err := websocket.DefaultDialer.DialContext(c.ctx, url, header)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.reconnCount = 0
	c.mu.Unlock()

	c.logger.Info("Connected to backend successfully")

	// Send registration message
	if err := c.register(); err != nil {
		c.logger.Errorf("Failed to register: %v", err)
		conn.Close()
		return err
	}

	// Trigger callback
	if c.onConnect != nil {
		c.onConnect()
	}

	return nil
}

// register sends registration message to backend
func (c *Client) register() error {
	msg := NewRegisterMessage(
		c.config.RobotID,
		c.config.Exchange,
		c.config.Version,
		c.config.TenantID,
	)

	data, err := msg.ToJSON()
	if err != nil {
		return err
	}

	c.logger.Infof("Sending actor.register: robot_id=%s, exchange=%s, version=%s, tenant_id=%d, data=%s",
		c.config.RobotID, c.config.Exchange, c.config.Version, c.config.TenantID, string(data))

	return c.writeMessage(data)
}

// Run starts the client read/write loops
func (c *Client) Run() {
	go c.writePump()
	go c.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.handleDisconnect(nil)
	}()

	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Errorf("WebSocket read error: %v", err)
				}
				c.handleDisconnect(err)
				return
			}

			c.handleMessage(message)
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return

		case message, ok := <-c.sendChan:
			if !ok {
				return
			}
			if err := c.writeMessage(message); err != nil {
				c.logger.Errorf("Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			// Send heartbeat
			if err := c.sendHeartbeat(); err != nil {
				c.logger.Errorf("Failed to send heartbeat: %v", err)
				return
			}
		}
	}
}

// writeMessage writes a message to the WebSocket connection
func (c *Client) writeMessage(data []byte) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(data []byte) {
	c.logger.Infof("Received message from backend: %s", string(data))

	// Try to parse as a command (from backend)
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Errorf("Failed to parse message: %v", err)
		return
	}

	// Check for pong response (text-based heartbeat response)
	// This resets the read deadline similar to WebSocket protocol pong
	if msgType, ok := msg.Data["type"].(string); ok && msgType == "pong" {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn != nil {
			conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		}
		c.logger.Debugf("Received pong, reset read deadline")
		return
	}

	// Check if it's a command for actor
	switch msg.Action {
	case ActionStart, ActionStop, ActionStatus, ActionConfig:
		c.handleCommand(&msg)
	default:
		c.logger.Infof("Received message: action=%s, data=%v", msg.Action, msg.Data)
	}
}

// handleCommand processes a command from backend
func (c *Client) handleCommand(msg *Message) {
	if c.handler == nil {
		c.logger.Warn("No command handler registered")
		return
	}

	cmd := &Command{
		Action:    msg.Action,
		Data:      msg.Data,
		RequestID: msg.RequestID,
	}

	c.logger.Infof("Handling command: action=%s, request_id=%s", cmd.Action, cmd.RequestID)

	// Execute command
	result := c.handler.HandleCommand(cmd)

	// Send result back to backend
	resultMsg := NewCommandResultMessage(result.RequestID, result.Success, result.Result, result.Error)
	data, err := resultMsg.ToJSON()
	if err != nil {
		c.logger.Errorf("Failed to marshal command result: %v", err)
		return
	}

	c.Send(data)
}

// handleDisconnect handles disconnection
func (c *Client) handleDisconnect(err error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return
	}
	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	c.logger.Warnf("Disconnected from backend: %v", err)

	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}

	// Attempt reconnection
	go c.reconnect()
}

// Send sends a message to backend
func (c *Client) Send(data []byte) error {
	select {
	case c.sendChan <- data:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		return fmt.Errorf("send channel full")
	}
}

// SendStatus sends a status update to backend
func (c *Client) SendStatus(status string, balance float64) error {
	msg := NewStatusUpdateMessage(c.config.RobotID, status, balance)
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}
	return c.Send(data)
}

// SendServerSync sends server info to backend
func (c *Client) SendServerSync(serverData *ServerSyncData) error {
	if serverData.RobotID == "" {
		serverData.RobotID = c.config.RobotID
	}
	msg := NewServerSyncMessage(serverData)
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}

	c.logger.Infof("Sending actor.server_sync: robot_id=%s, ip=%s, machine_id=%s, data=%s",
		serverData.RobotID, serverData.IP, serverData.MachineID, string(data))

	return c.Send(data)
}

// sendHeartbeat sends a heartbeat message
func (c *Client) sendHeartbeat() error {
	msg := NewHeartbeatMessage(c.config.RobotID)
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}
	return c.writeMessage(data)
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Close closes the client connection
func (c *Client) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Send unregister message
		msg := NewMessage(ActionUnregister, map[string]interface{}{
			"robot_id": c.config.RobotID,
		})
		if data, err := msg.ToJSON(); err == nil {
			c.conn.WriteMessage(websocket.TextMessage, data)
		}

		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	return nil
}
