package handler

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// RobotSyncService defines the interface for robot sync operations
type RobotSyncService interface {
	SyncRobot(ctx context.Context, tenantID uint32, rid string, status uint, balance float64) error
}

// RobotSyncHandler handles robot synchronization messages
type RobotSyncHandler struct {
	service RobotSyncService
	manager *websocket.Manager
	log     *log.Helper
}

// NewRobotSyncHandler creates a new robot sync handler
func NewRobotSyncHandler(service RobotSyncService, manager *websocket.Manager, logger log.Logger) *RobotSyncHandler {
	return &RobotSyncHandler{
		service: service,
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/robot_sync")),
	}
}

// Handle processes robot sync messages
func (h *RobotSyncHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	// Extract robot sync data
	rid, ok := msg.Data["rid"].(string)
	if !ok {
		h.log.Error("Missing or invalid 'rid' field")
		resp := protocol.NewErrorResponse(400, "Missing or invalid 'rid' field")
		return client.SendResponse(resp, msg.Action)
	}

	status, ok := msg.Data["status"].(float64)
	if !ok {
		h.log.Error("Missing or invalid 'status' field")
		resp := protocol.NewErrorResponse(400, "Missing or invalid 'status' field")
		return client.SendResponse(resp, msg.Action)
	}

	balance, ok := msg.Data["balance"].(float64)
	if !ok {
		h.log.Error("Missing or invalid 'balance' field")
		resp := protocol.NewErrorResponse(400, "Missing or invalid 'balance' field")
		return client.SendResponse(resp, msg.Action)
	}

	// Sync robot data
	if err := h.service.SyncRobot(client.Context(), client.TenantID, rid, uint(status), balance); err != nil {
		h.log.Errorf("Failed to sync robot %s: %v", rid, err)
		resp := protocol.NewErrorResponse(500, "Failed to sync robot")
		return client.SendResponse(resp, msg.Action)
	}

	h.log.Infof("Robot synced: rid=%s, status=%d, balance=%.2f, tenant=%d", rid, uint(status), balance, client.TenantID)

	// Broadcast update to other clients in the same tenant
	broadcastData := map[string]interface{}{
		"rid":     rid,
		"status":  uint(status),
		"balance": balance,
	}

	adapter := protocol.NewAdapter()
	for _, protocolType := range []protocol.ProtocolType{protocol.ProtocolTypeActor, protocol.ProtocolTypeLegacy} {
		resp := protocol.NewSuccessResponse(broadcastData)
		data, err := adapter.ConvertResponse(resp, protocolType, msg.Action)
		if err != nil {
			h.log.Errorf("Failed to convert broadcast message: %v", err)
			continue
		}

		// Broadcast to tenant (excluding the sender)
		h.manager.BroadcastToTenant(client.TenantID, data)
	}

	// Send success response to sender (no response for legacy protocol as per source)
	// But for Actor protocol, we should send a response
	if client.GetProtocolType() == protocol.ProtocolTypeActor {
		resp := protocol.NewSuccessResponse(nil)
		return client.SendResponse(resp, msg.Action)
	}

	return nil
}
