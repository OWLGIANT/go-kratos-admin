package handler

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// RobotSyncService 机器人同步服务接口
type RobotSyncService interface {
	SyncRobot(ctx context.Context, tenantID uint32, rid string, status uint, balance float64) error
}

// RobotSyncHandler 机器人同步处理器
type RobotSyncHandler struct {
	service RobotSyncService
	manager *websocket.Manager
	log     *log.Helper
}

// NewRobotSyncHandler 创建新的机器人同步处理器
func NewRobotSyncHandler(service RobotSyncService, manager *websocket.Manager, logger log.Logger) *RobotSyncHandler {
	return &RobotSyncHandler{
		service: service,
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/robot_sync")),
	}
}

// Handle 处理机器人同步消息
func (h *RobotSyncHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.RobotSyncCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in robot sync")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request
	syncedCount := int32(0)

	// 同步每个机器人
	for _, robot := range req.Robots {
		status := uint(0)
		switch robot.Status {
		case "running":
			status = 1
		case "stopped":
			status = 2
		case "error":
			status = 3
		}

		if err := h.service.SyncRobot(client.Context(), client.TenantID, robot.RobotID, status, 0); err != nil {
			h.log.Errorf("Failed to sync robot %s: %v", robot.RobotID, err)
			continue
		}
		syncedCount++
	}

	h.log.Infof("Robots synced: count=%d, tenant=%d", syncedCount, client.TenantID)

	// 广播更新给租户内其他客户端
	broadcastCmd := protocol.NewCommand(protocol.CommandTypeRobotSync)
	broadcastCmd.Payload = &protocol.RobotSyncCmd{
		Request: req,
	}
	h.manager.BroadcastCommandToTenant(client.TenantID, broadcastCmd)

	// 发送成功响应
	respPayload := &protocol.RobotSyncCmd{
		Response: &protocol.RobotSyncResponse{
			Success:     true,
			SyncedCount: syncedCount,
		},
	}
	return client.SendResponse(protocol.CommandTypeRobotSync, cmd.RequestID, cmd.Seq, respPayload)
}
