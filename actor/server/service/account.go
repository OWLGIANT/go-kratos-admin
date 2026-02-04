package service

import (
	"context"
	"fmt"
)

// Balance represents account balance
type Balance struct {
	Exchange  string             `json:"exchange"`
	Assets    map[string]float64 `json:"assets"`     // asset -> amount
	Frozen    map[string]float64 `json:"frozen"`     // asset -> frozen amount
	Timestamp int64              `json:"timestamp"`
}

// Position represents a trading position
type Position struct {
	Exchange      string  `json:"exchange"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`           // "long" or "short"
	Amount        float64 `json:"amount"`
	AvgPrice      float64 `json:"avg_price"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
	Leverage      int     `json:"leverage"`
	Timestamp     int64   `json:"timestamp"`
}

// Fee represents trading fee information
type Fee struct {
	Exchange  string  `json:"exchange"`
	TakerFee  float64 `json:"taker_fee"`
	MakerFee  float64 `json:"maker_fee"`
	Timestamp int64   `json:"timestamp"`
}

// AccountService handles account-related operations
type AccountService struct {
	manager *ExchangeManager
}

// NewAccountService creates a new account service
func NewAccountService(manager *ExchangeManager) *AccountService {
	return &AccountService{
		manager: manager,
	}
}

// GetBalance retrieves account balance
func (s *AccountService) GetBalance(ctx context.Context, exchange string) (*Balance, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Get account summary
	acctSum, apiErr := client.Rs.GetAcctSum()
	if !apiErr.Nil() {
		return nil, fmt.Errorf("failed to get account summary: %s", apiErr.Error())
	}

	// Convert to balance response
	balance := &Balance{
		Exchange:  exchange,
		Assets:    make(map[string]float64),
		Frozen:    make(map[string]float64),
		Timestamp: 0, // AcctSum doesn't have timestamp
	}

	// Add balance information
	for _, bal := range acctSum.Balances {
		balance.Assets[bal.Name] = bal.Amount
		balance.Frozen[bal.Name] = bal.Amount - bal.Avail
	}

	return balance, nil
}

// GetPositions retrieves all positions
func (s *AccountService) GetPositions(ctx context.Context, exchange string) ([]*Position, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Get account summary
	acctSum, apiErr := client.Rs.GetAcctSum()
	if !apiErr.Nil() {
		return nil, fmt.Errorf("failed to get account summary: %s", apiErr.Error())
	}

	// Convert positions
	positions := make([]*Position, 0)

	for _, pos := range acctSum.Positions {
		// Determine side based on amount (positive = long, negative = short)
		side := "long"
		amount := pos.Amount
		if amount < 0 {
			side = "short"
			amount = -amount
		}

		if amount > 0 {
			positions = append(positions, &Position{
				Exchange:      exchange,
				Symbol:        pos.Name,
				Side:          side,
				Amount:        amount,
				AvgPrice:      pos.Ave,
				UnrealizedPnl: pos.Pnl,
				Timestamp:     0,
			})
		}
	}

	return positions, nil
}

// GetFee retrieves trading fee information
func (s *AccountService) GetFee(ctx context.Context, exchange string) (*Fee, error) {
	// Get exchange client
	client, err := s.manager.GetExchange(exchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Get fee from exchange
	feeInfo, apiErr := client.Rs.GetFee()
	if !apiErr.Nil() {
		return nil, fmt.Errorf("failed to get fee: %s", apiErr.Error())
	}

	fee := &Fee{
		Exchange: exchange,
		TakerFee: feeInfo.Taker,
		MakerFee: feeInfo.Maker,
	}

	return fee, nil
}
