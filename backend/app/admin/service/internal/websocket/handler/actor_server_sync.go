package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorServerSyncHandler Actor 服务器同步处理器
type ActorServerSyncHandler struct {
	registry         *ActorRegistry
	manager          *websocket.Manager
	actorListHandler *ActorListHandler
	log              *log.Helper
}

// NewActorServerSyncHandler 创建新的 Actor 服务器同步处理器
func NewActorServerSyncHandler(registry *ActorRegistry, manager *websocket.Manager, actorListHandler *ActorListHandler, logger log.Logger) *ActorServerSyncHandler {
	return &ActorServerSyncHandler{
		registry:         registry,
		manager:          manager,
		actorListHandler: actorListHandler,
		log:              log.NewHelper(log.With(logger, "module", "websocket/handler/actor_server_sync")),
	}
}

// Handle 处理 Actor 服务器同步消息
func (h *ActorServerSyncHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	h.log.Infof("Received server.sync: client=%s, isActor=%v", client.ID, client.IsActor)

	payload, ok := cmd.Payload.(*protocol.ServerSyncCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in server sync")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request
	robotID := req.RobotID
	if robotID == "" {
		// 尝试从客户端获取机器人 ID
		if client.IsActor && client.RobotID != "" {
			robotID = client.RobotID
		} else {
			h.log.Error("Missing robot_id in server sync")
			return client.SendError(cmd.RequestID, cmd.Seq, 400, "Missing robot_id")
		}
	}

	// 更新 Actor 的服务器信息
	if !h.registry.UpdateServerInfo(robotID, req.ServerInfo, req.IP, req.InnerIP, req.Port, req.MachineID, req.Nickname) {
		h.log.Warnf("Actor not found for server sync: robot_id=%s", robotID)
		return client.SendError(cmd.RequestID, cmd.Seq, 404, "Actor not found")
	}

	h.log.Infof("Actor server info synced: robot_id=%s, ip=%s, machine_id=%s", robotID, req.IP, req.MachineID)

	// 广播更新后的 Actor 列表给所有客户端
	if h.actorListHandler != nil {
		h.actorListHandler.BroadcastActorList()
	}

	// 发送成功响应
	respPayload := &protocol.ServerSyncCmd{
		Response: &protocol.ServerSyncResponse{
			Success: true,
		},
	}
	return client.SendResponse(protocol.CommandTypeServerSync, cmd.RequestID, cmd.Seq, respPayload)
}
