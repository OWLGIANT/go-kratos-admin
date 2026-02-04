package websocket

import (
	"errors"
	"sync"

	"actor/third/log"
)

// Pool WebSocket 连接池
type Pool struct {
	clients map[string]*Client
	mu      sync.RWMutex
	logger  log.Logger
}

// NewPool 创建新的连接池
func NewPool() *Pool {
	return &Pool{
		clients: make(map[string]*Client),
		logger:  log.RootLogger,
	}
}

// Add 添加客户端到连接池
func (p *Pool) Add(name string, client *Client) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.clients[name]; exists {
		return errors.New("client already exists")
	}

	p.clients[name] = client
	p.logger.Infof("Added client '%s' to pool", name)
	return nil
}

// Get 从连接池获取客户端
func (p *Pool) Get(name string) (*Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, exists := p.clients[name]
	if !exists {
		return nil, errors.New("client not found")
	}

	return client, nil
}

// Remove 从连接池移除客户端
func (p *Pool) Remove(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, exists := p.clients[name]
	if !exists {
		return errors.New("client not found")
	}

	// 断开连接
	if client.IsConnected() {
		client.Disconnect()
	}

	delete(p.clients, name)
	p.logger.Infof("Removed client '%s' from pool", name)
	return nil
}

// GetAll 获取所有客户端
func (p *Pool) GetAll() map[string]*Client {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clients := make(map[string]*Client, len(p.clients))
	for name, client := range p.clients {
		clients[name] = client
	}

	return clients
}

// DisconnectAll 断开所有连接
func (p *Pool) DisconnectAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, client := range p.clients {
		if client.IsConnected() {
			client.Disconnect()
			p.logger.Infof("Disconnected client '%s'", name)
		}
	}
}

// Clear 清空连接池
func (p *Pool) Clear() {
	p.DisconnectAll()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.clients = make(map[string]*Client)
	p.logger.Info("Cleared connection pool")
}

// Count 返回连接池中的客户端数量
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// ConnectedCount 返回已连接的客户端数量
func (p *Pool) ConnectedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, client := range p.clients {
		if client.IsConnected() {
			count++
		}
	}

	return count
}
