package protocol

import (
	"encoding/json"
	"fmt"
)

// LegacyAdapter handles conversion between Legacy protocol and UnifiedMessage
type LegacyAdapter struct{}

// NewLegacyAdapter creates a new legacy protocol adapter
func NewLegacyAdapter() *LegacyAdapter {
	return &LegacyAdapter{}
}

// ToUnified converts a legacy request to a unified message
func (a *LegacyAdapter) ToUnified(data []byte) (*UnifiedMessage, error) {
	var req LegacyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legacy request: %w", err)
	}

	// Map protocol number to action
	action, ok := ProtocolToAction[req.Data.ProtocolNumber]
	if !ok {
		return nil, fmt.Errorf("unknown protocol number: %d", req.Data.ProtocolNumber)
	}

	// Parse message body into map
	var messageData map[string]any
	if len(req.Data.MessageBody) > 0 {
		if err := json.Unmarshal(req.Data.MessageBody, &messageData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message body: %w", err)
		}
	}

	return &UnifiedMessage{
		Action: action,
		Data:   messageData,
	}, nil
}

// FromUnified converts a unified response to legacy response format
func (a *LegacyAdapter) FromUnified(resp *UnifiedResponse, action string) ([]byte, error) {
	// Map action to protocol number
	protocolNumber, ok := ActionToProtocol[action]
	if !ok {
		return nil, fmt.Errorf("unknown action: %s", action)
	}

	legacyResp := LegacyResponse{
		ProtocolNumber: protocolNumber,
		MessageBody:    resp.Data,
	}

	return json.Marshal(legacyResp)
}

// IsLegacyProtocol checks if the data is in legacy protocol format
func IsLegacyProtocol(data []byte) bool {
	var req LegacyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return false
	}

	// Check if it has the legacy structure
	return req.Data.ProtocolNumber > 0
}
