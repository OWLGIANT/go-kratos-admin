package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// Client WebSocket 客户端连接
type Client struct {
	// 连接唯一标识
	ID string

	// 用户信息
	UserID   uint32
	TenantID uint32
	Username string

	// Actor 信息（用于 actor 客户端）
	IsActor bool
	RobotID string

	// WebSocket 连接
	conn *websocket.Conn

	// 内容类型
	contentType protocol.ContentType

	// 编解码器
	codec protocol.Codec

	// 发送通道
	send chan []byte

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 互斥锁
	mu sync.RWMutex

	// 最后活动时间
	lastActivity time.Time

	// 序列号计数器
	seqCounter uint64

	// Manager 引用
	manager *Manager

	// 确保 Close 只调用一次
	closeOnce sync.Once
}

// NewClient 创建新的 WebSocket 客户端
func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		ID:           uuid.New().String(),
		conn:         conn,
		contentType:  protocol.ContentTypeJSON,
		codec:        protocol.NewAutoCodec(),
		send:         make(chan []byte, 256),
		ctx:          ctx,
		cancel:       cancel,
		lastActivity: time.Now(),
		manager:      manager,
	}
}

// SetUserInfo 设置用户信息
func (c *Client) SetUserInfo(userID, tenantID uint32, username string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.UserID = userID
	c.TenantID = tenantID
	c.Username = username
}

// SetActorInfo 设置 Actor 信息
func (c *Client) SetActorInfo(robotID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.IsActor = true
	c.RobotID = robotID
}

// SetContentType 设置内容类型
func (c *Client) SetContentType(contentType protocol.ContentType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.contentType = contentType
	switch contentType {
	case protocol.ContentTypeJSON:
		c.codec = protocol.NewJSONCodec()
	default:
		c.codec = protocol.NewAutoCodec()
	}
}

// GetContentType 获取内容类型
func (c *Client) GetContentType() protocol.ContentType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.contentType
}

// SetCodec 设置编解码器
func (c *Client) SetCodec(codec protocol.Codec) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.codec = codec
}

// GetCodec 获取编解码器
func (c *Client) GetCodec() protocol.Codec {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.codec
}

// UpdateActivity 更新最后活动时间
func (c *Client) UpdateActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastActivity = time.Now()
}

// GetLastActivity 获取最后活动时间
func (c *Client) GetLastActivity() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastActivity
}

// NextSeq 获取下一个序列号
func (c *Client) NextSeq() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seqCounter++
	return c.seqCounter
}

// SendMessage 发送原始消息
func (c *Client) SendMessage(data []byte) error {
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		// 通道已满，客户端太慢
		return ErrClientSlow
	}
}

// SendCommand 发送命令
func (c *Client) SendCommand(cmd *protocol.Command) error {
	c.mu.RLock()
	codec := c.codec
	c.mu.RUnlock()

	data, err := codec.Encode(cmd)
	if err != nil {
		return err
	}

	return c.SendMessage(data)
}

// SendResponse 发送响应命令
func (c *Client) SendResponse(cmdType protocol.CommandType, requestID string, seq uint64, payload interface{}) error {
	cmd := &protocol.Command{
		Type:      cmdType,
		RequestID: requestID,
		Seq:       seq,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	return c.SendCommand(cmd)
}

// SendError 发送错误响应
func (c *Client) SendError(requestID string, seq uint64, code int32, message string) error {
	cmd := protocol.NewErrorCommand(code, message)
	cmd.RequestID = requestID
	cmd.Seq = seq
	return c.SendCommand(cmd)
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.send)
		err = c.conn.Close()
	})
	return err
}

// ReadPump 从 WebSocket 连接读取消息
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
				// 记录意外关闭
			}
			break
		}

		// 重置读取超时
		c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		c.UpdateActivity()

		// 通过 manager 处理消息
		c.manager.HandleMessage(c, message)
	}
}

// WritePump 向 WebSocket 连接写入消息
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
				// 通道已关闭
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

// Context 返回客户端上下文
func (c *Client) Context() context.Context {
	return c.ctx
}
