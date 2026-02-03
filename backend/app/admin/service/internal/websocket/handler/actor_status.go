package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorStatusHandler handles actor status update messages
type ActorStatusHandler struct {
	registry *ActorRegistry
	log      *log.Helper
}

// NewActorStatusHandler creates a new actor status handler
func NewActorStatusHandler(registry *ActorRegistry, logger log.Logger) *ActorStatusHandler {
	return &ActorStatusHandler{
		registry: registry,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_status")),
	}
}

// Handle processes actor status update messages
func (h *ActorStatusHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	robotID, _ := msg.Data["robot_id"].(string)
	if robotID == "" {
		h.log.Error("Missing robot_id in status update")
		resp := protocol.NewErrorResponse(400, "Missing robot_id")
		return client.SendResponse(resp, msg.Action)
	}

	status, _ := msg.Data["status"].(string)
	balance := 0.0
	if b, ok := msg.Data["balance"].(float64); ok {
		balance = b
	}

	// Update actor status in registry
	if !h.registry.UpdateStatus(robotID, status, balance) {
		h.log.Warnf("Actor not found for status update: robot_id=%s", robotID)
		resp := protocol.NewErrorResponse(404, "Actor not found")
		return client.SendResponse(resp, msg.Action)
	}

	h.log.Infof("Actor status updated: robot_id=%s, status=%s, balance=%.2f", robotID, status, balance)

	// Send success response
	resp := protocol.NewSuccessResponse(nil)
	return client.SendResponse(resp, msg.Action)
}

// HandleHeartbeat processes actor heartbeat messages
func (h *ActorStatusHandler) HandleHeartbeat(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	robotID, _ := msg.Data["robot_id"].(string)
	if robotID == "" {
		// Try to get robot ID from registry by client ID
		info := h.registry.GetByClientID(client.ID)
		if info != nil {
			robotID = info.RobotID
		}
	}

	if robotID != "" {
		h.registry.UpdateHeartbeat(robotID)
		h.log.Debugf("Actor heartbeat: robot_id=%s", robotID)
	}

	// Update client activity
	client.UpdateActivity()

	// Send pong response
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"type": "pong",
	})
	return client.SendResponse(resp, msg.Action)
}
