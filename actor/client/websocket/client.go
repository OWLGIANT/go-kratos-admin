package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"actor/third/log"
	"github.com/gorilla/websocket"
)

// MessageHandler 消息处理函数类型
type MessageHandler func(messageType int, data []byte) error

// ErrorHandler 错误处理函数类型
type ErrorHandler func(err error)

// Config WebSocket 客户端配置
type Config struct {
	URL                string        // WebSocket 服务器地址
	ReconnectInterval  time.Duration // 重连间隔
	MaxReconnectCount  int           // 最大重连次数，0表示无限重连
	PingInterval       time.Duration // 心跳间隔
	PongWait           time.Duration // 等待pong响应的超时时间
	WriteWait          time.Duration // 写入超时时间
	ReadBufferSize     int           // 读缓冲区大小
	WriteBufferSize    int           // 写缓冲区大小
	EnableCompression  bool          // 是否启用压缩
	HandshakeTimeout   time.Duration // 握手超时时间
}

// DefaultConfig 返回默认配置
func DefaultConfig(url string) *Config {
	return &Config{
		URL:                url,
		ReconnectInterval:  5 * time.Second,
		MaxReconnectCount:  0, // 无限重连
		PingInterval:       30 * time.Second,
		PongWait:           60 * time.Second,
		WriteWait:          10 * time.Second,
		ReadBufferSize:     1024,
		WriteBufferSize:    1024,
		EnableCompression:  false,
		HandshakeTimeout:   10 * time.Second,
	}
}

// Client WebSocket 客户端
type Client struct {
	config          *Config
	conn            *websocket.Conn
	mu              sync.RWMutex
	connected       bool
	reconnecting    bool
	reconnectCount  int
	messageHandler  MessageHandler
	errorHandler    ErrorHandler
	sendChan        chan []byte
	closeChan       chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	onConnect       func()
	onDisconnect    func()
}

// NewClient 创建新的 WebSocket 客户端
func NewClient(config *Config) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		config:    config,
		sendChan:  make(chan []byte, 100),
		closeChan: make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetMessageHandler 设置消息处理函数
func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.messageHandler = handler
}

// SetErrorHandler 设置错误处理函数
func (c *Client) SetErrorHandler(handler ErrorHandler) {
	c.errorHandler = handler
}

// SetOnConnect 设置连接成功回调
func (c *Client) SetOnConnect(callback func()) {
	c.onConnect = callback
}

// SetOnDisconnect 设置断开连接回调
func (c *Client) SetOnDisconnect(callback func()) {
	c.onDisconnect = callback
}

// Connect 连接到 WebSocket 服务器
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return errors.New("already connected")
	}

	dialer := websocket.Dialer{
		ReadBufferSize:    c.config.ReadBufferSize,
		WriteBufferSize:   c.config.WriteBufferSize,
		EnableCompression: c.config.EnableCompression,
		HandshakeTimeout:  c.config.HandshakeTimeout,
	}

	conn, _, err := dialer.Dial(c.config.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.connected = true
	c.reconnectCount = 0

	log.RootLogger.Infof("WebSocket connected to %s", c.config.URL)

	// 启动读写协程
	go c.readPump()
	go c.writePump()
	go c.pingPump()

	if c.onConnect != nil {
		c.onConnect()
	}

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return errors.New("not connected")
	}

	c.cancel()
	close(c.closeChan)

	if c.conn != nil {
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.RootLogger.Errorf("Error sending close message: %v", err)
		}
		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	log.RootLogger.Info("WebSocket disconnected")

	if c.onDisconnect != nil {
		c.onDisconnect()
	}

	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Send 发送消息
func (c *Client) Send(data []byte) error {
	if !c.IsConnected() {
		return errors.New("not connected")
	}

	select {
	case c.sendChan <- data:
		return nil
	case <-time.After(c.config.WriteWait):
		return errors.New("send timeout")
	}
}

// SendJSON 发送 JSON 消息
func (c *Client) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	return c.Send(data)
}

// readPump 读取消息
func (c *Client) readPump() {
	defer func() {
		c.handleDisconnect()
	}()

	if c.config.PongWait > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		c.conn.SetPongHandler(func(string) error {
			c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
			return nil
		})
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.RootLogger.Errorf("WebSocket read error: %v", err)
					if c.errorHandler != nil {
						c.errorHandler(err)
					}
				}
				return
			}

			if c.messageHandler != nil {
				if err := c.messageHandler(messageType, message); err != nil {
					log.RootLogger.Errorf("Message handler error: %v", err)
					if c.errorHandler != nil {
						c.errorHandler(err)
					}
				}
			}
		}
	}
}

// writePump 写入消息
func (c *Client) writePump() {
	defer func() {
		c.handleDisconnect()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.sendChan:
			if !ok {
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.RootLogger.Errorf("WebSocket write error: %v", err)
				if c.errorHandler != nil {
					c.errorHandler(err)
				}
				return
			}
		}
	}
}

// pingPump 发送心跳
func (c *Client) pingPump() {
	if c.config.PingInterval <= 0 {
		return
	}

	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if !c.IsConnected() {
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.RootLogger.Errorf("WebSocket ping error: %v", err)
				return
			}
		}
	}
}

// handleDisconnect 处理断开连接
func (c *Client) handleDisconnect() {
	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	c.mu.Unlock()

	if wasConnected && c.onDisconnect != nil {
		c.onDisconnect()
	}

	// 尝试重连
	if c.config.MaxReconnectCount == 0 || c.reconnectCount < c.config.MaxReconnectCount {
		c.reconnect()
	} else {
		log.RootLogger.Errorf("Max reconnect attempts reached")
	}
}

// reconnect 重新连接
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.reconnectCount++
			log.RootLogger.Infof("Attempting to reconnect (%d)...", c.reconnectCount)

			// 清理旧连接
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}

			// 尝试重新连接
			dialer := websocket.Dialer{
				ReadBufferSize:    c.config.ReadBufferSize,
				WriteBufferSize:   c.config.WriteBufferSize,
				EnableCompression: c.config.EnableCompression,
				HandshakeTimeout:  c.config.HandshakeTimeout,
			}

			conn, _, err := dialer.Dial(c.config.URL, nil)
			if err != nil {
				log.RootLogger.Errorf("Reconnect failed: %v", err)
				if c.config.MaxReconnectCount > 0 && c.reconnectCount >= c.config.MaxReconnectCount {
					log.RootLogger.Errorf("Max reconnect attempts reached")
					return
				}
				time.Sleep(c.config.ReconnectInterval)
				continue
			}

			c.mu.Lock()
			c.conn = conn
			c.connected = true
			c.reconnectCount = 0
			c.mu.Unlock()

			log.RootLogger.Infof("WebSocket reconnected to %s", c.config.URL)

			// 重新启动读写协程
			go c.readPump()
			go c.writePump()
			go c.pingPump()

			if c.onConnect != nil {
				c.onConnect()
			}

			return
		}
	}
}
