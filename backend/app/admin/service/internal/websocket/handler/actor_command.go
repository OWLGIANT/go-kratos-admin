package handler

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// PendingCommand represents a command waiting for response
type PendingCommand struct {
	RequestID  string
	Action     string
	RobotID    string
	SentAt     time.Time
	ResultChan chan *CommandResultData
}

// CommandResultData represents the result of a command
type CommandResultData struct {
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Result    interface{} `json:"result,omitempty"`
}

// ActorCommandHandler handles sending commands to actors and receiving results
type ActorCommandHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper

	// Pending commands waiting for response
	pending   map[string]*PendingCommand
	pendingMu sync.RWMutex

	// Command timeout
	timeout time.Duration
}

// NewActorCommandHandler creates a new actor command handler
func NewActorCommandHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorCommandHandler {
	return &ActorCommandHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_command")),
		pending:  make(map[string]*PendingCommand),
		timeout:  30 * time.Second,
	}
}

// SetTimeout sets the command timeout
func (h *ActorCommandHandler) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// SendCommand sends a command to an actor and waits for response
func (h *ActorCommandHandler) SendCommand(robotID, action string, data map[string]interface{}) (*CommandResultData, error) {
	// Get actor info
	info := h.registry.Get(robotID)
	if info == nil {
		return &CommandResultData{
			Success: false,
			Error:   "actor not found",
		}, nil
	}

	// Get client
	client, err := h.manager.GetClient(info.ClientID)
	if err != nil {
		return &CommandResultData{
			Success: false,
			Error:   "actor client not connected",
		}, nil
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Create pending command
	pending := &PendingCommand{
		RequestID:  requestID,
		Action:     action,
		RobotID:    robotID,
		SentAt:     time.Now(),
		ResultChan: make(chan *CommandResultData, 1),
	}

	// Register pending command
	h.pendingMu.Lock()
	h.pending[requestID] = pending
	h.pendingMu.Unlock()

	// Cleanup on exit
	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, requestID)
		h.pendingMu.Unlock()
	}()

	// Build command message
	msg := &protocol.ActorMessage{
		Action:    action,
		Data:      data,
		RequestID: requestID,
		Timestamp: time.Now().Unix(),
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return &CommandResultData{
			Success: false,
			Error:   "failed to marshal command",
		}, nil
	}

	// Send command
	if err := client.SendMessage(msgData); err != nil {
		return &CommandResultData{
			Success: false,
			Error:   "failed to send command",
		}, nil
	}

	h.log.Infof("Command sent: robot_id=%s, action=%s, request_id=%s", robotID, action, requestID)

	// Wait for response with timeout
	select {
	case result := <-pending.ResultChan:
		return result, nil
	case <-time.After(h.timeout):
		return &CommandResultData{
			RequestID: requestID,
			Success:   false,
			Error:     "command timeout",
		}, nil
	}
}

// Handle processes command result messages from actors
func (h *ActorCommandHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	requestID, _ := msg.Data["request_id"].(string)
	if requestID == "" {
		h.log.Warn("Missing request_id in command result")
		return nil
	}

	// Get pending command
	h.pendingMu.RLock()
	pending, ok := h.pending[requestID]
	h.pendingMu.RUnlock()

	if !ok {
		h.log.Warnf("No pending command found for request_id=%s", requestID)
		return nil
	}

	// Build result
	result := &CommandResultData{
		RequestID: requestID,
	}

	if success, ok := msg.Data["success"].(bool); ok {
		result.Success = success
	}
	if errMsg, ok := msg.Data["error"].(string); ok {
		result.Error = errMsg
	}
	if res, ok := msg.Data["result"]; ok {
		result.Result = res
	}

	h.log.Infof("Command result received: request_id=%s, success=%v", requestID, result.Success)

	// Send result to waiting goroutine
	select {
	case pending.ResultChan <- result:
	default:
		h.log.Warnf("Result channel full for request_id=%s", requestID)
	}

	return nil
}

// SendStartCommand sends a start command to an actor
func (h *ActorCommandHandler) SendStartCommand(robotID string, config map[string]interface{}) (*CommandResultData, error) {
	return h.SendCommand(robotID, "actor.start", config)
}

// SendStopCommand sends a stop command to an actor
func (h *ActorCommandHandler) SendStopCommand(robotID string, graceful bool, reason string) (*CommandResultData, error) {
	return h.SendCommand(robotID, "actor.stop", map[string]interface{}{
		"graceful": graceful,
		"reason":   reason,
	})
}

// SendStatusCommand sends a status query command to an actor
func (h *ActorCommandHandler) SendStatusCommand(robotID string) (*CommandResultData, error) {
	return h.SendCommand(robotID, "actor.status", nil)
}

// SendConfigCommand sends a config update command to an actor
func (h *ActorCommandHandler) SendConfigCommand(robotID string, config map[string]interface{}) (*CommandResultData, error) {
	return h.SendCommand(robotID, "actor.config", config)
}

// GetPendingCommands returns all pending commands
func (h *ActorCommandHandler) GetPendingCommands() []*PendingCommand {
	h.pendingMu.RLock()
	defer h.pendingMu.RUnlock()

	result := make([]*PendingCommand, 0, len(h.pending))
	for _, cmd := range h.pending {
		result = append(result, cmd)
	}
	return result
}

// CleanupExpiredCommands removes expired pending commands
func (h *ActorCommandHandler) CleanupExpiredCommands() {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	now := time.Now()
	for id, cmd := range h.pending {
		if now.Sub(cmd.SentAt) > h.timeout {
			// Send timeout result
			select {
			case cmd.ResultChan <- &CommandResultData{
				RequestID: cmd.RequestID,
				Success:   false,
				Error:     "command timeout (cleanup)",
			}:
			default:
			}
			delete(h.pending, id)
		}
	}
}
