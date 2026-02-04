package websocket

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

var (
	// ErrClientSlow 客户端接收消息太慢
	ErrClientSlow = errors.New("client is too slow")

	// ErrClientNotFound 客户端未找到
	ErrClientNotFound = errors.New("client not found")

	// ErrMaxConnectionsReached 达到最大连接数
	ErrMaxConnectionsReached = errors.New("max connections reached")

	// ErrMaxConnectionsPerUserReached 达到每用户最大连接数
	ErrMaxConnectionsPerUserReached = errors.New("max connections per user reached")
)

// CommandHandler 命令处理器接口
type CommandHandler interface {
	HandleCommand(client *Client, cmd *protocol.Command) error
}

// Manager 管理所有 WebSocket 客户端连接
type Manager struct {
	// 所有连接的客户端 (clientID -> Client)
	clients sync.Map

	// 用户到客户端的映射 (userID -> map[clientID]bool)
	userClients sync.Map

	// 命令处理器
	commandHandler CommandHandler

	// 编解码器
	codec *protocol.AutoCodec

	// 配置
	maxConnections        int
	maxConnectionsPerUser int

	// 日志
	log *log.Helper

	// 统计
	mu               sync.RWMutex
	totalClients     int
	totalUserClients map[uint32]int
}

// NewManager 创建新的连接管理器
func NewManager(logger log.Logger, maxConnections, maxConnectionsPerUser int) *Manager {
	return &Manager{
		codec:                 protocol.NewAutoCodec(),
		maxConnections:        maxConnections,
		maxConnectionsPerUser: maxConnectionsPerUser,
		log:                   log.NewHelper(log.With(logger, "module", "websocket/manager")),
		totalUserClients:      make(map[uint32]int),
	}
}

// SetCommandHandler 设置命令处理器
func (m *Manager) SetCommandHandler(handler CommandHandler) {
	m.commandHandler = handler
}

// Register 注册新的客户端连接
func (m *Manager) Register(client *Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查最大连接数
	if m.maxConnections > 0 && m.totalClients >= m.maxConnections {
		return ErrMaxConnectionsReached
	}

	// 检查每用户最大连接数
	if m.maxConnectionsPerUser > 0 && client.UserID > 0 {
		userCount := m.totalUserClients[client.UserID]
		if userCount >= m.maxConnectionsPerUser {
			return ErrMaxConnectionsPerUserReached
		}
	}

	// 注册客户端
	m.clients.Store(client.ID, client)
	m.totalClients++

	// 注册用户映射
	if client.UserID > 0 {
		userClientsMap, _ := m.userClients.LoadOrStore(client.UserID, &sync.Map{})
		userClientsMap.(*sync.Map).Store(client.ID, true)
		m.totalUserClients[client.UserID]++
	}

	m.log.Infof("Client registered: id=%s, userID=%d, tenantID=%d, total=%d",
		client.ID, client.UserID, client.TenantID, m.totalClients)

	return nil
}

// Unregister 注销客户端连接
func (m *Manager) Unregister(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 注销客户端
	if _, ok := m.clients.LoadAndDelete(client.ID); ok {
		m.totalClients--
	}

	// 注销用户映射
	if client.UserID > 0 {
		if userClientsMap, ok := m.userClients.Load(client.UserID); ok {
			userClientsMap.(*sync.Map).Delete(client.ID)
			m.totalUserClients[client.UserID]--

			// 清理空的用户映射
			if m.totalUserClients[client.UserID] == 0 {
				m.userClients.Delete(client.UserID)
				delete(m.totalUserClients, client.UserID)
			}
		}
	}

	m.log.Infof("Client unregistered: id=%s, userID=%d, tenantID=%d, total=%d",
		client.ID, client.UserID, client.TenantID, m.totalClients)
}

// GetClient 根据 ID 获取客户端
func (m *Manager) GetClient(clientID string) (*Client, error) {
	if client, ok := m.clients.Load(clientID); ok {
		return client.(*Client), nil
	}
	return nil, ErrClientNotFound
}

// GetUserClients 获取用户的所有客户端
func (m *Manager) GetUserClients(userID uint32) []*Client {
	var clients []*Client

	if userClientsMap, ok := m.userClients.Load(userID); ok {
		userClientsMap.(*sync.Map).Range(func(key, value interface{}) bool {
			if client, err := m.GetClient(key.(string)); err == nil {
				clients = append(clients, client)
			}
			return true
		})
	}

	return clients
}

// Broadcast 向所有连接的客户端广播消息
func (m *Manager) Broadcast(data []byte) {
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if err := client.SendMessage(data); err != nil {
			m.log.Warnf("Failed to send broadcast to client %s: %v", client.ID, err)
		}
		return true
	})
}

// BroadcastCommand 向所有连接的客户端广播命令
func (m *Manager) BroadcastCommand(cmd *protocol.Command) {
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if err := client.SendCommand(cmd); err != nil {
			m.log.Warnf("Failed to send broadcast command to client %s: %v", client.ID, err)
		}
		return true
	})
}

// BroadcastToUser 向特定用户的所有连接广播消息
func (m *Manager) BroadcastToUser(userID uint32, data []byte) {
	clients := m.GetUserClients(userID)
	for _, client := range clients {
		if err := client.SendMessage(data); err != nil {
			m.log.Warnf("Failed to send message to user %d client %s: %v", userID, client.ID, err)
		}
	}
}

// BroadcastCommandToUser 向特定用户的所有连接广播命令
func (m *Manager) BroadcastCommandToUser(userID uint32, cmd *protocol.Command) {
	clients := m.GetUserClients(userID)
	for _, client := range clients {
		if err := client.SendCommand(cmd); err != nil {
			m.log.Warnf("Failed to send command to user %d client %s: %v", userID, client.ID, err)
		}
	}
}

// BroadcastToTenant 向特定租户的所有连接广播消息
func (m *Manager) BroadcastToTenant(tenantID uint32, data []byte) {
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.TenantID == tenantID {
			if err := client.SendMessage(data); err != nil {
				m.log.Warnf("Failed to send message to tenant %d client %s: %v", tenantID, client.ID, err)
			}
		}
		return true
	})
}

// BroadcastCommandToTenant 向特定租户的所有连接广播命令
func (m *Manager) BroadcastCommandToTenant(tenantID uint32, cmd *protocol.Command) {
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.TenantID == tenantID {
			if err := client.SendCommand(cmd); err != nil {
				m.log.Warnf("Failed to send command to tenant %d client %s: %v", tenantID, client.ID, err)
			}
		}
		return true
	})
}

// HandleMessage 处理来自客户端的消息
func (m *Manager) HandleMessage(client *Client, data []byte) {
	// 解码消息
	cmd, err := m.codec.Decode(data)
	if err != nil {
		m.log.Errorf("Failed to decode message for client %s: %v", client.ID, err)
		// 发送错误响应
		errCmd := protocol.NewErrorCommand(400, "Invalid message format")
		client.SendCommand(errCmd)
		return
	}

	// 通过处理器处理命令
	if m.commandHandler != nil {
		if err := m.commandHandler.HandleCommand(client, cmd); err != nil {
			m.log.Errorf("Failed to handle command for client %s: %v", client.ID, err)
			// 发送错误响应
			errCmd := protocol.NewErrorCommand(500, "Internal server error")
			errCmd.RequestID = cmd.RequestID
			errCmd.Seq = cmd.Seq
			client.SendCommand(errCmd)
		}
	}
}

// GetStats 获取连接统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_clients":   m.totalClients,
		"total_users":     len(m.totalUserClients),
		"max_connections": m.maxConnections,
		"max_per_user":    m.maxConnectionsPerUser,
	}
}

// KickUser 踢出特定用户的所有连接
func (m *Manager) KickUser(userID uint32, reason string) {
	clients := m.GetUserClients(userID)
	for _, client := range clients {
		// 发送踢出消息
		cmd := protocol.NewCommand(protocol.CommandTypeUserKick)
		cmd.Payload = &protocol.UserKickCmd{
			Request: &protocol.UserKickRequest{
				UserID: string(rune(userID)),
				Reason: reason,
			},
		}
		client.SendCommand(cmd)

		// 关闭连接
		m.Unregister(client)
		client.Close()
	}

	m.log.Infof("Kicked user %d: %s (%d connections)", userID, reason, len(clients))
}

// KickClient 踢出特定客户端
func (m *Manager) KickClient(clientID string, reason string) error {
	client, err := m.GetClient(clientID)
	if err != nil {
		return err
	}

	// 发送踢出消息
	cmd := protocol.NewCommand(protocol.CommandTypeUserKick)
	cmd.Payload = &protocol.UserKickCmd{
		Request: &protocol.UserKickRequest{
			Reason: reason,
		},
	}
	client.SendCommand(cmd)

	// 关闭连接
	m.Unregister(client)
	client.Close()

	m.log.Infof("Kicked client %s: %s", clientID, reason)
	return nil
}

// GetAllClients 获取所有客户端
func (m *Manager) GetAllClients() []*Client {
	var clients []*Client
	m.clients.Range(func(key, value interface{}) bool {
		clients = append(clients, value.(*Client))
		return true
	})
	return clients
}

// GetActorClients 获取所有 Actor 客户端
func (m *Manager) GetActorClients() []*Client {
	var clients []*Client
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.IsActor {
			clients = append(clients, client)
		}
		return true
	})
	return clients
}

// SendCommandToClient 向特定客户端发送命令
func (m *Manager) SendCommandToClient(clientID string, cmd *protocol.Command) error {
	client, err := m.GetClient(clientID)
	if err != nil {
		return err
	}
	return client.SendCommand(cmd)
}

// SendCommandToActor 向特定 Actor 发送命令
func (m *Manager) SendCommandToActor(robotID string, cmd *protocol.Command) error {
	var targetClient *Client
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.IsActor && client.RobotID == robotID {
			targetClient = client
			return false
		}
		return true
	})

	if targetClient == nil {
		return ErrClientNotFound
	}

	return targetClient.SendCommand(cmd)
}

// CleanupInactiveClients 清理不活跃的客户端
func (m *Manager) CleanupInactiveClients(timeout time.Duration) int {
	var toRemove []*Client
	now := time.Now()

	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if now.Sub(client.GetLastActivity()) > timeout {
			toRemove = append(toRemove, client)
		}
		return true
	})

	for _, client := range toRemove {
		m.log.Infof("Cleaning up inactive client: %s", client.ID)
		m.Unregister(client)
		client.Close()
	}

	return len(toRemove)
}
