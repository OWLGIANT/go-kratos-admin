package service

import (
	"context"
	"fmt"
	"time"

	"actor/helper"
)

// Ticker represents market ticker data
type Ticker struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	Last      float64 `json:"last"`
	Bid       float64 `json:"bid"`
	Ask       float64 `json:"ask"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

// Depth represents market depth/orderbook
type Depth struct {
	Exchange  string          `json:"exchange"`
	Symbol    string          `json:"symbol"`
	Bids      [][]float64     `json:"bids"` // [[price, amount], ...]
	Asks      [][]float64     `json:"asks"` // [[price, amount], ...]
	Timestamp int64           `json:"timestamp"`
}

// Kline represents candlestick data
type Kline struct {
	Symbol    string  `json:"symbol"`
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// GetKlineRequest represents a request to get kline data
type GetKlineRequest struct {
	Exchange  string `json:"exchange"`
	Symbol    string `json:"symbol"`
	Interval  string `json:"interval"`  // "1m", "5m", "15m", "1h", "4h", "1d"
	StartTime int64  `json:"start_time,omitempty"`
	EndTime   int64  `json:"end_time,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// MarketService handles market data operations
type MarketService struct {
	manager *ExchangeManager
}

// NewMarketService creates a new market service
func NewMarketService(manager *ExchangeManager) *MarketService {
	return &MarketService{
		manager: manager,
	}
}

// GetTicker retrieves ticker data for a symbol
func (s *MarketService) GetTicker(ctx context.Context, exchange, symbol string) (*Ticker, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Parse symbol to pair
	pair, err := helper.StringPairToPair(symbol)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol: %w", err)
	}

	// Create signal to get ticker
	signal := helper.Signal{
		Pair: pair,
		Type: helper.SignalTypeGetTicker,
	}

	// Send signal to get ticker
	client.Rs.SendSignal([]helper.Signal{signal})

	// Note: In a real implementation, you would wait for the callback
	// or use a synchronous method. For now, return a placeholder.
	ticker := &Ticker{
		Exchange:  exchange,
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
	}

	return ticker, nil
}

// GetDepth retrieves orderbook depth for a symbol
func (s *MarketService) GetDepth(ctx context.Context, exchange, symbol string) (*Depth, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Get depth from exchange
	respDepth, apiErr := client.Rs.GetDepthByPair(symbol)
	if !apiErr.Nil() {
		return nil, fmt.Errorf("failed to get depth: %s", apiErr.Error())
	}

	// Convert to response format
	depth := &Depth{
		Exchange:  exchange,
		Symbol:    symbol,
		Bids:      make([][]float64, 0, len(respDepth.Bids)),
		Asks:      make([][]float64, 0, len(respDepth.Asks)),
		Timestamp: respDepth.Seq.Inner.Load(),
	}

	// Convert bids
	for _, bid := range respDepth.Bids {
		depth.Bids = append(depth.Bids, []float64{bid.Price, bid.Amount})
	}

	// Convert asks
	for _, ask := range respDepth.Asks {
		depth.Asks = append(depth.Asks, []float64{ask.Price, ask.Amount})
	}

	return depth, nil
}

// GetKline retrieves candlestick data
func (s *MarketService) GetKline(ctx context.Context, req *GetKlineRequest) ([]*Kline, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(req.Exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Note: The base Rs interface doesn't have a GetKline method
	// This would need to be implemented using the Do() method or
	// by adding a GetKline method to the Rs interface

	// For now, return an error indicating this needs implementation
	_ = client
	return nil, fmt.Errorf("GetKline not yet implemented - requires exchange-specific implementation")
}
