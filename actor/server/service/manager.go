package service

import (
	"fmt"
	"sync"

	"actor/broker/base"
	"actor/broker/ex/binance_spot"
	"actor/broker/ex/binance_usdt_swap"
	"actor/broker/ex/bitget_spot"
	"actor/broker/ex/bitget_usdt_swap"
	"actor/broker/ex/okx_spot"
	"actor/broker/ex/okx_usdt_swap"
	"actor/helper"
	"actor/third/log"
)

// ExchangeClient represents a single exchange connection
type ExchangeClient struct {
	Rs     base.Rs
	Ws     base.Ws
	Config helper.BrokerConfig
	Name   string
}

// ExchangeManager manages multiple exchange connections
type ExchangeManager struct {
	exchanges map[string]*ExchangeClient // key: exchange name (e.g., "binance_usdt_swap")
	mu        sync.RWMutex
	callbacks helper.CallbackFunc
	logger    log.Logger
}

// NewExchangeManager creates a new exchange manager
func NewExchangeManager(logger log.Logger) *ExchangeManager {
	return &ExchangeManager{
		exchanges: make(map[string]*ExchangeClient),
		logger:    logger,
	}
}

// SetCallbacks sets the callback function for all exchanges
func (m *ExchangeManager) SetCallbacks(cb helper.CallbackFunc) {
	m.callbacks = cb
}

// InitExchange initializes an exchange connection
func (m *ExchangeManager) InitExchange(name string, config helper.BrokerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already initialized
	if _, exists := m.exchanges[name]; exists {
		return fmt.Errorf("exchange %s already initialized", name)
	}

	// Set logger if not provided
	if config.Logger == nil {
		config.Logger = m.logger
	}
	if config.RootLogger == nil {
		config.RootLogger = m.logger
	}

	// Create BrokerConfigExt
	configExt := &helper.BrokerConfigExt{
		BrokerConfig: config,
	}

	// Create TradeMsg for callbacks
	msg := &helper.TradeMsg{}

	// Get first pair info (for initialization)
	var pairInfo *helper.ExchangeInfo
	if len(config.Pairs) > 0 {
		// In a real implementation, you would fetch exchange info
		// For now, we'll pass nil and let the exchange handle it
		pairInfo = nil
	}

	// Create Rs and Ws based on exchange name
	var rs base.Rs
	var ws base.Ws

	switch name {
	case "binance_usdt_swap":
		rs = binance_usdt_swap.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = binance_usdt_swap.NewWs(configExt, msg, pairInfo, m.callbacks)
	case "binance_spot":
		rs = binance_spot.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = binance_spot.NewWs(configExt, msg, pairInfo, m.callbacks)
	case "okx_usdt_swap":
		rs = okx_usdt_swap.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = okx_usdt_swap.NewWs(configExt, msg, pairInfo, m.callbacks)
	case "okx_spot":
		rs = okx_spot.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = okx_spot.NewWs(configExt, msg, pairInfo, m.callbacks)
	case "bitget_usdt_swap":
		rs = bitget_usdt_swap.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = bitget_usdt_swap.NewWs(configExt, msg, pairInfo, m.callbacks)
	case "bitget_spot":
		rs = bitget_spot.NewRs(configExt, msg, pairInfo, m.callbacks)
		ws = bitget_spot.NewWs(configExt, msg, pairInfo, m.callbacks)
	default:
		return fmt.Errorf("unsupported exchange: %s", name)
	}

	// Start WebSocket connection
	if ws != nil {
		go ws.Run()
	}

	// Store the client
	client := &ExchangeClient{
		Rs:     rs,
		Ws:     ws,
		Config: config,
		Name:   name,
	}

	m.exchanges[name] = client
	m.logger.Infof("Exchange %s initialized successfully", name)

	return nil
}

// GetExchange retrieves an exchange client by name
func (m *ExchangeManager) GetExchange(name string) (*ExchangeClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.exchanges[name]
	if !exists {
		return nil, fmt.Errorf("exchange %s not found", name)
	}

	return client, nil
}

// GetAllExchanges returns all initialized exchanges
func (m *ExchangeManager) GetAllExchanges() map[string]*ExchangeClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid concurrent modification
	result := make(map[string]*ExchangeClient, len(m.exchanges))
	for k, v := range m.exchanges {
		result[k] = v
	}

	return result
}

// Stop stops all exchange connections
func (m *ExchangeManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.exchanges {
		m.logger.Infof("Stopping exchange %s", name)
		if client.Rs != nil {
			client.Rs.Stop()
		}
		if client.Ws != nil {
			client.Ws.Stop()
		}
	}

	m.exchanges = make(map[string]*ExchangeClient)
}
