package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// ActorAdapter handles conversion between Actor protocol and UnifiedMessage
type ActorAdapter struct{}

// NewActorAdapter creates a new actor protocol adapter
func NewActorAdapter() *ActorAdapter {
	return &ActorAdapter{}
}

// ToUnified converts an actor message to a unified message
func (a *ActorAdapter) ToUnified(data []byte) (*UnifiedMessage, error) {
	var msg ActorMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actor message: %w", err)
	}

	// Handle ping/heartbeat messages
	action := msg.Action
	if msg.Type == "ping" {
		action = "heartbeat"
	}

	return &UnifiedMessage{
		Action:    action,
		Data:      msg.Data,
		RequestID: msg.RequestID,
		Timestamp: msg.Timestamp,
	}, nil
}

// FromUnified converts a unified response to actor response format
func (a *ActorAdapter) FromUnified(resp *UnifiedResponse, action string) ([]byte, error) {
	actorResp := ActorResponse{
		Success:   resp.Success,
		Code:      resp.Code,
		Message:   resp.Message,
		Data:      resp.Data,
		RequestID: resp.RequestID,
		Timestamp: time.Now().Unix(),
	}

	return json.Marshal(actorResp)
}

// IsActorProtocol checks if the data is in actor protocol format
func IsActorProtocol(data []byte) bool {
	var msg ActorMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}

	// Check if it has the actor structure (type field or action field)
	return msg.Type != "" || msg.Action != ""
}
