package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorListHandler Actor 列表处理器
type ActorListHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper
}

// NewActorListHandler 创建新的 Actor 列表处理器
func NewActorListHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorListHandler {
	return &ActorListHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_list")),
	}
}

// Handle 处理 Actor 列表请求
func (h *ActorListHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	h.log.Infof("Received actor.list request: client=%s, isActor=%v", client.ID, client.IsActor)

	payload, _ := cmd.Payload.(*protocol.ActorListCmd)

	var tenantID uint32
	var status string
	if payload != nil && payload.Request != nil {
		tenantID = payload.Request.TenantID
		status = payload.Request.Status
	}

	// 获取 Actor 列表
	var actors []*ActorInfo
	if tenantID > 0 {
		actors = h.registry.GetByTenant(tenantID)
	} else {
		actors = h.registry.GetAll()
	}

	// 按状态过滤
	if status != "" {
		filtered := make([]*ActorInfo, 0)
		for _, actor := range actors {
			if actor.Status == status {
				filtered = append(filtered, actor)
			}
		}
		actors = filtered
	}

	h.log.Infof("Actor list requested: client=%s, count=%d", client.ID, len(actors))

	// 转换为协议格式
	protoActors := make([]*protocol.ActorInfo, len(actors))
	for i, actor := range actors {
		protoActors[i] = &protocol.ActorInfo{
			ClientID:      actor.ClientID,
			RobotID:       actor.RobotID,
			Exchange:      actor.Exchange,
			Version:       actor.Version,
			TenantID:      actor.TenantID,
			Status:        actor.Status,
			Balance:       actor.Balance,
			RegisteredAt:  actor.RegisteredAt,
			LastHeartbeat: actor.LastHeartbeat,
			ServerInfo:    actor.ServerInfo,
			IP:            actor.IP,
			InnerIP:       actor.InnerIP,
			Port:          actor.Port,
			MachineID:     actor.MachineID,
			Nickname:      actor.Nickname,
		}
	}

	// 发送响应
	respPayload := &protocol.ActorListCmd{
		Response: &protocol.ActorListResponse{
			Actors: protoActors,
			Total:  int32(len(protoActors)),
		},
	}
	return client.SendResponse(protocol.CommandTypeActorList, cmd.RequestID, cmd.Seq, respPayload)
}

// BroadcastActorList 广播 Actor 列表给所有客户端
func (h *ActorListHandler) BroadcastActorList() {
	actors := h.registry.GetAll()

	// 转换为协议格式
	protoActors := make([]*protocol.ActorInfo, len(actors))
	for i, actor := range actors {
		protoActors[i] = &protocol.ActorInfo{
			ClientID:      actor.ClientID,
			RobotID:       actor.RobotID,
			Exchange:      actor.Exchange,
			Version:       actor.Version,
			TenantID:      actor.TenantID,
			Status:        actor.Status,
			Balance:       actor.Balance,
			RegisteredAt:  actor.RegisteredAt,
			LastHeartbeat: actor.LastHeartbeat,
			ServerInfo:    actor.ServerInfo,
			IP:            actor.IP,
			InnerIP:       actor.InnerIP,
			Port:          actor.Port,
			MachineID:     actor.MachineID,
			Nickname:      actor.Nickname,
		}
	}

	cmd := protocol.NewCommand(protocol.CommandTypeActorList)
	cmd.Payload = &protocol.ActorListCmd{
		Response: &protocol.ActorListResponse{
			Actors: protoActors,
			Total:  int32(len(protoActors)),
		},
	}

	h.manager.BroadcastCommand(cmd)
	h.log.Infof("Actor list broadcasted: count=%d", len(actors))
}
