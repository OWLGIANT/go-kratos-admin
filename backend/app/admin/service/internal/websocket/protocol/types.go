package protocol

import (
	"encoding/json"
	"time"
)

// ProtocolType represents the type of protocol being used
type ProtocolType string

const (
	ProtocolTypeLegacy ProtocolType = "legacy"
	ProtocolTypeActor  ProtocolType = "actor"
)

// UnifiedMessage represents a protocol-agnostic message
type UnifiedMessage struct {
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// UnifiedResponse represents a protocol-agnostic response
type UnifiedResponse struct {
	Success   bool                   `json:"success"`
	Code      int                    `json:"code"`
	Message   string                 `json:"message"`
	Data      interface{}            `json:"data,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// Legacy Protocol Types (WsRequest/WsResponse)

// LegacyRequest represents the legacy protocol request format
// Note: The router actually parses WsResponse format from clients
type LegacyRequest struct {
	Code int              `json:"code"`
	Data LegacyRequestData `json:"data"`
	Msg  string           `json:"msg"`
}

type LegacyRequestData struct {
	ProtocolNumber int             `json:"protocol_number"`
	MessageBody    json.RawMessage `json:"message_body"`
}

// LegacyResponse represents the legacy protocol response format
type LegacyResponse struct {
	ProtocolNumber int         `json:"protocol_number"`
	MessageBody    interface{} `json:"message_body"`
}

// Actor Protocol Types (Message/Response)

// ActorMessage represents the actor protocol message format
type ActorMessage struct {
	Type      string                 `json:"type"`
	Action    string                 `json:"action,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// ActorResponse represents the actor protocol response format
type ActorResponse struct {
	Success   bool        `json:"success"`
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// Protocol Number to Action Mapping
const (
	ProtocolHeartbeat  = 99999
	ProtocolRobotSync  = 10003
	ProtocolAlert      = 10002
	ProtocolKick       = 10001
)

var (
	ProtocolToAction = map[int]string{
		ProtocolHeartbeat: "heartbeat",
		ProtocolRobotSync: "robot.sync",
		ProtocolAlert:     "alert.send",
		ProtocolKick:      "auth.kick",
	}

	ActionToProtocol = map[string]int{
		"heartbeat":   ProtocolHeartbeat,
		"robot.sync":  ProtocolRobotSync,
		"alert.send":  ProtocolAlert,
		"auth.kick":   ProtocolKick,
	}
)

// Helper functions

// NewUnifiedResponse creates a new unified response
func NewUnifiedResponse(success bool, code int, message string, data interface{}) *UnifiedResponse {
	return &UnifiedResponse{
		Success:   success,
		Code:      code,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(data interface{}) *UnifiedResponse {
	return NewUnifiedResponse(true, 0, "success", data)
}

// NewErrorResponse creates an error response
func NewErrorResponse(code int, message string) *UnifiedResponse {
	return NewUnifiedResponse(false, code, message, nil)
}
