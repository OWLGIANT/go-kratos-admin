package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// AlertHandler handles alert/notification messages
type AlertHandler struct {
	manager *websocket.Manager
	log     *log.Helper
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(manager *websocket.Manager, logger log.Logger) *AlertHandler {
	return &AlertHandler{
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/alert")),
	}
}

// Handle processes alert messages
func (h *AlertHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	// Extract alert data
	message, ok := msg.Data["message"].(string)
	if !ok {
		h.log.Error("Missing or invalid 'message' field")
		resp := protocol.NewErrorResponse(400, "Missing or invalid 'message' field")
		return client.SendResponse(resp, msg.Action)
	}

	// Optional: target user ID for directed alerts
	var targetUserID uint32
	if userID, ok := msg.Data["user_id"].(float64); ok {
		targetUserID = uint32(userID)
	}

	// Optional: alert level
	level := "info"
	if l, ok := msg.Data["level"].(string); ok {
		level = l
	}

	alertData := map[string]interface{}{
		"message": message,
		"level":   level,
		"from":    client.Username,
	}

	h.log.Infof("Alert received: message=%s, level=%s, from=%s, target=%d", message, level, client.Username, targetUserID)

	// Send alert to target user or broadcast to tenant
	adapter := protocol.NewAdapter()
	resp := protocol.NewSuccessResponse(alertData)

	if targetUserID > 0 {
		// Send to specific user
		for _, protocolType := range []protocol.ProtocolType{protocol.ProtocolTypeActor, protocol.ProtocolTypeLegacy} {
			data, err := adapter.ConvertResponse(resp, protocolType, msg.Action)
			if err != nil {
				h.log.Errorf("Failed to convert alert message: %v", err)
				continue
			}
			h.manager.BroadcastToUser(targetUserID, data)
		}
	} else {
		// Broadcast to entire tenant
		for _, protocolType := range []protocol.ProtocolType{protocol.ProtocolTypeActor, protocol.ProtocolTypeLegacy} {
			data, err := adapter.ConvertResponse(resp, protocolType, msg.Action)
			if err != nil {
				h.log.Errorf("Failed to convert alert message: %v", err)
				continue
			}
			h.manager.BroadcastToTenant(client.TenantID, data)
		}
	}

	// Send success response to sender
	successResp := protocol.NewSuccessResponse(nil)
	return client.SendResponse(successResp, msg.Action)
}
