package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// KickHandler handles kicking/disconnecting users
type KickHandler struct {
	manager *websocket.Manager
	log     *log.Helper
}

// NewKickHandler creates a new kick handler
func NewKickHandler(manager *websocket.Manager, logger log.Logger) *KickHandler {
	return &KickHandler{
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/kick")),
	}
}

// Handle processes kick messages
func (h *KickHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	// Extract target user ID
	targetUserID, ok := msg.Data["user_id"].(float64)
	if !ok {
		h.log.Error("Missing or invalid 'user_id' field")
		resp := protocol.NewErrorResponse(400, "Missing or invalid 'user_id' field")
		return client.SendResponse(resp, msg.Action)
	}

	// Optional: reason for kicking
	reason := "You have been logged out"
	if r, ok := msg.Data["reason"].(string); ok {
		reason = r
	}

	h.log.Infof("Kicking user %d: %s (requested by %s)", uint32(targetUserID), reason, client.Username)

	// Kick the user (close all their connections)
	h.manager.KickUser(uint32(targetUserID), reason)

	// Send success response to sender
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"user_id": uint32(targetUserID),
		"kicked":  true,
	})
	return client.SendResponse(resp, msg.Action)
}
