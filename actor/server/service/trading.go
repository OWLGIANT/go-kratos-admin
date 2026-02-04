package service

import (
	"context"
	"fmt"
	"time"

	"actor/helper"
	"actor/third/fixed"
)

// PlaceOrderRequest represents a request to place an order
type PlaceOrderRequest struct {
	Exchange string  `json:"exchange"` // e.g., "binance_usdt_swap"
	Symbol   string  `json:"symbol"`   // e.g., "BTC_USDT"
	Side     string  `json:"side"`     // "buy" or "sell"
	Type     string  `json:"type"`     // "limit" or "market"
	Price    float64 `json:"price"`
	Amount   float64 `json:"amount"`
	ClientID string  `json:"client_id,omitempty"` // Optional client order ID
}

// CancelOrderRequest represents a request to cancel an order
type CancelOrderRequest struct {
	Exchange string `json:"exchange"`
	Symbol   string `json:"symbol"`
	OrderID  string `json:"order_id,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// GetOrderListRequest represents a request to get order list
type GetOrderListRequest struct {
	Exchange   string `json:"exchange"`
	Symbol     string `json:"symbol,omitempty"`
	StartTime  int64  `json:"start_time,omitempty"` // milliseconds
	EndTime    int64  `json:"end_time,omitempty"`   // milliseconds
	OrderState string `json:"order_state,omitempty"` // "all", "pending", "filled", "canceled"
}

// GetOrderDetailRequest represents a request to get order detail
type GetOrderDetailRequest struct {
	Exchange string `json:"exchange"`
	Symbol   string `json:"symbol"`
	OrderID  string `json:"order_id,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// OrderResponse represents an order response
type OrderResponse struct {
	OrderID    string  `json:"order_id"`
	ClientID   string  `json:"client_id,omitempty"`
	Exchange   string  `json:"exchange"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Type       string  `json:"type"`
	Price      float64 `json:"price"`
	Amount     float64 `json:"amount"`
	Filled     float64 `json:"filled"`
	Status     string  `json:"status"`
	CreateTime int64   `json:"create_time"`
	UpdateTime int64   `json:"update_time,omitempty"`
}

// TradingService handles trading operations
type TradingService struct {
	manager *ExchangeManager
}

// NewTradingService creates a new trading service
func NewTradingService(manager *ExchangeManager) *TradingService {
	return &TradingService{
		manager: manager,
	}
}

// PlaceOrder places a new order
func (s *TradingService) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (*OrderResponse, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(req.Exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Parse symbol to pair
	pair, err := helper.StringPairToPair(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol: %w", err)
	}

	// Convert side
	var side helper.OrderSide
	switch req.Side {
	case "buy":
		side = helper.OrderSideKD // 开多
	case "sell":
		side = helper.OrderSideKK // 开空
	default:
		return nil, fmt.Errorf("invalid side: %s", req.Side)
	}

	// Convert type
	var orderType helper.OrderType
	switch req.Type {
	case "limit":
		orderType = helper.OrderTypeLimit
	case "market":
		orderType = helper.OrderTypeMarket
	default:
		return nil, fmt.Errorf("invalid order type: %s", req.Type)
	}

	// Create signal
	signal := helper.Signal{
		Pair:      pair,
		OrderSide: side,
		Type:      helper.SignalTypeNewOrder,
		OrderType: orderType,
		Price:     req.Price,
		Amount:    fixed.NewF(req.Amount),
		ClientID:  req.ClientID,
	}

	// Send signal
	client.Rs.SendSignal([]helper.Signal{signal})

	// For now, return a basic response
	// In a real implementation, you would wait for the callback or query the order
	resp := &OrderResponse{
		ClientID:   req.ClientID,
		Exchange:   req.Exchange,
		Symbol:     req.Symbol,
		Side:       req.Side,
		Type:       req.Type,
		Price:      req.Price,
		Amount:     req.Amount,
		Status:     "pending",
		CreateTime: time.Now().UnixMilli(),
	}

	return resp, nil
}

// CancelOrder cancels an existing order
func (s *TradingService) CancelOrder(ctx context.Context, req *CancelOrderRequest) error {
	// Get exchange client
	client, err := s.manager.GetExchange(req.Exchange)
	if err != nil {
		return fmt.Errorf("failed to get exchange: %w", err)
	}

	// Parse symbol to pair
	pair, err := helper.StringPairToPair(req.Symbol)
	if err != nil {
		return fmt.Errorf("invalid symbol: %w", err)
	}

	// Create cancel signal
	signal := helper.Signal{
		Pair:     pair,
		Type:     helper.SignalTypeCancelOrder,
		OrderID:  req.OrderID,
		ClientID: req.ClientID,
	}

	// Send signal
	client.Rs.SendSignal([]helper.Signal{signal})

	return nil
}

// GetOrderList retrieves a list of orders
func (s *TradingService) GetOrderList(ctx context.Context, req *GetOrderListRequest) ([]*OrderResponse, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(req.Exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Set default time range if not provided
	startTime := req.StartTime
	endTime := req.EndTime
	if startTime == 0 {
		startTime = time.Now().Add(-24 * time.Hour).UnixMilli()
	}
	if endTime == 0 {
		endTime = time.Now().UnixMilli()
	}

	// Convert order state
	var orderState helper.OrderState
	switch req.OrderState {
	case "pending":
		orderState = helper.OrderStatePending
	case "finished":
		orderState = helper.OrderStateFinished
	default:
		orderState = helper.OrderStateAll
	}

	// Get order list from exchange
	orders, apiErr := client.Rs.GetOrderList(startTime, endTime, orderState)
	if !apiErr.Nil() {
		return nil, fmt.Errorf("failed to get order list: %s", apiErr.Error())
	}

	// Convert to response format
	result := make([]*OrderResponse, 0, len(orders))
	for _, order := range orders {
		resp := &OrderResponse{
			OrderID:    order.OrderID,
			ClientID:   order.ClientID,
			Exchange:   req.Exchange,
			Symbol:     order.Symbol,
			Side:       order.OrderSide.String(),
			Type:       order.OrderType.String(),
			Price:      order.Price,
			Amount:     order.Amount.Float(),
			Filled:     order.Filled.Float(),
			Status:     order.OrderState.String(),
			CreateTime: order.CreatedTimeMs,
			UpdateTime: order.UpdatedTimeMs,
		}
		result = append(result, resp)
	}

	return result, nil
}

// GetOrderDetail retrieves details of a specific order
func (s *TradingService) GetOrderDetail(ctx context.Context, req *GetOrderDetailRequest) (*OrderResponse, error) {
	// Get order list and filter by order ID or client ID
	listReq := &GetOrderListRequest{
		Exchange:   req.Exchange,
		Symbol:     req.Symbol,
		OrderState: "all",
	}

	orders, err := s.GetOrderList(ctx, listReq)
	if err != nil {
		return nil, err
	}

	// Find the specific order
	for _, order := range orders {
		if (req.OrderID != "" && order.OrderID == req.OrderID) ||
			(req.ClientID != "" && order.ClientID == req.ClientID) {
			return order, nil
		}
	}

	return nil, fmt.Errorf("order not found")
}
