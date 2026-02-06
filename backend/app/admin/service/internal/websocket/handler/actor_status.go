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

	// 通过客户端 ID 获取 Actor 信息
	info := h.registry.GetByClientID(client.ID)
	if info == nil {
		h.log.Warnf("Actor not found for status update: client_id=%s", client.ID)
		return client.SendError(cmd.RequestID, cmd.Seq, 404, "Actor not found")
	}

	// 如果有服务器信息，更新
	if req.ServerInfo != nil {
		serverInfoMap := map[string]interface{}{
			"cpu":      req.ServerInfo.CPU,
			"ip_pool":  req.ServerInfo.IPPool,
			"mem":      req.ServerInfo.Mem,
			"mem_pct":  req.ServerInfo.MemPct,
			"disk_pct": req.ServerInfo.DiskPct,
			"task_num": req.ServerInfo.TaskNum,
		}
		h.registry.UpdateServerInfo(info.IP, serverInfoMap, "", "", "", "")
	}

	h.log.Infof("Actor status updated: ip=%s, status=%s", info.IP, req.Status)

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
