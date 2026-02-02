package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// HeartbeatHandler handles heartbeat/ping messages
type HeartbeatHandler struct {
	log *log.Helper
}

// NewHeartbeatHandler creates a new heartbeat handler
func NewHeartbeatHandler(logger log.Logger) *HeartbeatHandler {
	return &HeartbeatHandler{
		log: log.NewHelper(log.With(logger, "module", "websocket/handler/heartbeat")),
	}
}

// Handle processes heartbeat messages
func (h *HeartbeatHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	// Update client activity
	client.UpdateActivity()

	// Send pong response
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"type": "pong",
		"time": msg.Timestamp,
	})

	if err := client.SendResponse(resp, msg.Action); err != nil {
		h.log.Errorf("Failed to send heartbeat response to client %s: %v", client.ID, err)
		return err
	}

	h.log.Debugf("Heartbeat processed for client %s", client.ID)
	return nil
}
