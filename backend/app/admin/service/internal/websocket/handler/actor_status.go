package handler

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorStatusHandler Actor 状态处理器
type ActorStatusHandler struct {
	registry *ActorRegistry
	log      *log.Helper
}

// NewActorStatusHandler 创建新的 Actor 状态处理器
func NewActorStatusHandler(registry *ActorRegistry, logger log.Logger) *ActorStatusHandler {
	return &ActorStatusHandler{
		registry: registry,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_status")),
	}
}

// Handle 处理 Actor 状态更新命令
func (h *ActorStatusHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.ActorStatusCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in status update")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request
	if req.RobotID == "" {
		h.log.Error("Missing robot_id in status update")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Missing robot_id")
	}

	// 更新 Actor 状态
	if !h.registry.UpdateStatus(req.RobotID, req.Status, req.Balance) {
		h.log.Warnf("Actor not found for status update: robot_id=%s", req.RobotID)
		return client.SendError(cmd.RequestID, cmd.Seq, 404, "Actor not found")
	}

	// 如果有服务器信息，也更新
	if req.ServerInfo != nil {
		h.registry.UpdateServerInfo(req.RobotID, req.ServerInfo, "", "", "", "", "")
	}

	h.log.Infof("Actor status updated: robot_id=%s, status=%s, balance=%.2f", req.RobotID, req.Status, req.Balance)

	// 发送成功响应
	respPayload := &protocol.ActorStatusCmd{
		Response: &protocol.ActorStatusResponse{
			Acknowledged: true,
		},
	}
	return client.SendResponse(protocol.CommandTypeActorStatus, cmd.RequestID, cmd.Seq, respPayload)
}

// HandleHeartbeat 处理 Actor 心跳命令
func (h *ActorStatusHandler) HandleHeartbeat(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.ActorHeartbeatCmd)

	var robotID string
	if ok && payload.Request != nil {
		robotID = payload.Request.RobotID
	}

	if robotID == "" {
		// 尝试从注册表中通过客户端 ID 获取机器人 ID
		info := h.registry.GetByClientID(client.ID)
		if info != nil {
			robotID = info.RobotID
		}
	}

	if robotID != "" {
		h.registry.UpdateHeartbeat(robotID)
		h.log.Debugf("Actor heartbeat: robot_id=%s", robotID)
	}

	// 更新客户端活动时间
	client.UpdateActivity()

	// 发送心跳响应
	respPayload := &protocol.ActorHeartbeatCmd{
		Response: &protocol.ActorHeartbeatResponse{
			ServerTime: time.Now().UnixMilli(),
		},
	}
	return client.SendResponse(protocol.CommandTypeActorHeartbeat, cmd.RequestID, cmd.Seq, respPayload)
}
