package websocket

import (
	"errors"
	"sync"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

var (
	// ErrClientSlow is returned when a client is too slow to receive messages
	ErrClientSlow = errors.New("client is too slow")

	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")

	// ErrMaxConnectionsReached is returned when max connections limit is reached
	ErrMaxConnectionsReached = errors.New("max connections reached")

	// ErrMaxConnectionsPerUserReached is returned when max connections per user limit is reached
	ErrMaxConnectionsPerUserReached = errors.New("max connections per user reached")
)

// Manager manages all WebSocket client connections
type Manager struct {
	// All connected clients (clientID -> Client)
	clients sync.Map

	// User to clients mapping (userID -> map[clientID]bool)
	userClients sync.Map

	// Message handler
	messageHandler MessageHandler

	// Protocol adapter
	adapter *protocol.Adapter

	// Configuration
	maxConnections        int
	maxConnectionsPerUser int

	// Logger
	log *log.Helper

	// Metrics
	mu              sync.RWMutex
	totalClients    int
	totalUserClients map[uint32]int
}

// MessageHandler handles incoming messages
type MessageHandler interface {
	HandleMessage(client *Client, msg *protocol.UnifiedMessage) error
}

// NewManager creates a new connection manager
func NewManager(logger log.Logger, maxConnections, maxConnectionsPerUser int) *Manager {
	return &Manager{
		adapter:               protocol.NewAdapter(),
		maxConnections:        maxConnections,
		maxConnectionsPerUser: maxConnectionsPerUser,
		log:                   log.NewHelper(log.With(logger, "module", "websocket/manager")),
		totalUserClients:      make(map[uint32]int),
	}
}

// SetMessageHandler sets the message handler
func (m *Manager) SetMessageHandler(handler MessageHandler) {
	m.messageHandler = handler
}

// Register registers a new client connection
func (m *Manager) Register(client *Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check max connections
	if m.maxConnections > 0 && m.totalClients >= m.maxConnections {
		return ErrMaxConnectionsReached
	}

	// Check max connections per user
	if m.maxConnectionsPerUser > 0 && client.UserID > 0 {
		userCount := m.totalUserClients[client.UserID]
		if userCount >= m.maxConnectionsPerUser {
			return ErrMaxConnectionsPerUserReached
		}
	}

	// Register client
	m.clients.Store(client.ID, client)
	m.totalClients++

	// Register user mapping
	if client.UserID > 0 {
		userClientsMap, _ := m.userClients.LoadOrStore(client.UserID, &sync.Map{})
		userClientsMap.(*sync.Map).Store(client.ID, true)
		m.totalUserClients[client.UserID]++
	}

	m.log.Infof("Client registered: id=%s, userID=%d, tenantID=%d, total=%d",
		client.ID, client.UserID, client.TenantID, m.totalClients)

	return nil
}

// Unregister unregisters a client connection
func (m *Manager) Unregister(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unregister client
	if _, ok := m.clients.LoadAndDelete(client.ID); ok {
		m.totalClients--
	}

	// Unregister user mapping
	if client.UserID > 0 {
		if userClientsMap, ok := m.userClients.Load(client.UserID); ok {
			userClientsMap.(*sync.Map).Delete(client.ID)
			m.totalUserClients[client.UserID]--

			// Clean up empty user mapping
			if m.totalUserClients[client.UserID] == 0 {
				m.userClients.Delete(client.UserID)
				delete(m.totalUserClients, client.UserID)
			}
		}
	}

	m.log.Infof("Client unregistered: id=%s, userID=%d, tenantID=%d, total=%d",
		client.ID, client.UserID, client.TenantID, m.totalClients)
}

// GetClient returns a client by ID
func (m *Manager) GetClient(clientID string) (*Client, error) {
	if client, ok := m.clients.Load(clientID); ok {
		return client.(*Client), nil
	}
	return nil, ErrClientNotFound
}

// GetUserClients returns all clients for a user
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

// Broadcast sends a message to all connected clients
func (m *Manager) Broadcast(data []byte) {
	m.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if err := client.SendMessage(data); err != nil {
			m.log.Warnf("Failed to send broadcast to client %s: %v", client.ID, err)
		}
		return true
	})
}

// BroadcastToUser sends a message to all connections of a specific user
func (m *Manager) BroadcastToUser(userID uint32, data []byte) {
	clients := m.GetUserClients(userID)
	for _, client := range clients {
		if err := client.SendMessage(data); err != nil {
			m.log.Warnf("Failed to send message to user %d client %s: %v", userID, client.ID, err)
		}
	}
}

// BroadcastToTenant sends a message to all connections of a specific tenant
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

// HandleMessage handles an incoming message from a client
func (m *Manager) HandleMessage(client *Client, data []byte) {
	// Detect protocol and convert to unified message
	msg, protocolType, err := m.adapter.DetectAndConvert(data)
	if err != nil {
		m.log.Errorf("Failed to detect protocol for client %s: %v", client.ID, err)
		// Send error response
		resp := protocol.NewErrorResponse(400, "Invalid message format")
		client.SendResponse(resp, "")
		return
	}

	// Set protocol type for client (first message determines protocol)
	if client.GetProtocolType() == "" {
		client.SetProtocolType(protocolType)
	}

	// Handle message through handler
	if m.messageHandler != nil {
		if err := m.messageHandler.HandleMessage(client, msg); err != nil {
			m.log.Errorf("Failed to handle message for client %s: %v", client.ID, err)
			// Send error response
			resp := protocol.NewErrorResponse(500, "Internal server error")
			client.SendResponse(resp, msg.Action)
		}
	}
}

// GetStats returns connection statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_clients":      m.totalClients,
		"total_users":        len(m.totalUserClients),
		"max_connections":    m.maxConnections,
		"max_per_user":       m.maxConnectionsPerUser,
	}
}

// KickUser kicks all connections for a specific user
func (m *Manager) KickUser(userID uint32, reason string) {
	clients := m.GetUserClients(userID)
	for _, client := range clients {
		// Send kick message
		resp := protocol.NewErrorResponse(401, reason)
		client.SendResponse(resp, "auth.kick")

		// Close connection
		m.Unregister(client)
		client.Close()
	}

	m.log.Infof("Kicked user %d: %s (%d connections)", userID, reason, len(clients))
}
