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

	actors := h.registry.GetAll()

	h.log.Infof("Actor list requested: client=%s, count=%d", client.ID, len(actors))

	// 转换为协议格式
	protoActors := h.convertToProtoActors(actors)

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

	protoActors := h.convertToProtoActors(actors)

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

// convertToProtoActors 转换为协议格式
func (h *ActorListHandler) convertToProtoActors(actors []*ActorServerInfo) []*protocol.ActorInfo {
	protoActors := make([]*protocol.ActorInfo, len(actors))
	for i, actor := range actors {
		machineID := ""
		if actor.MachineID != nil {
			machineID = *actor.MachineID
		}

		// 转换 ServerInfo map 为 protocol.ServerStatusInfo
		var serverInfo *protocol.ServerStatusInfo
		if actor.ServerInfo != nil {
			serverInfo = &protocol.ServerStatusInfo{}
			if cpu, ok := actor.ServerInfo["cpu"].(string); ok {
				serverInfo.CPU = cpu
			}
			if ipPool, ok := actor.ServerInfo["ip_pool"].(float64); ok {
				serverInfo.IPPool = ipPool
			}
			if mem, ok := actor.ServerInfo["mem"].(float64); ok {
				serverInfo.Mem = mem
			}
			if memPct, ok := actor.ServerInfo["mem_pct"].(string); ok {
				serverInfo.MemPct = memPct
			}
			if diskPct, ok := actor.ServerInfo["disk_pct"].(string); ok {
				serverInfo.DiskPct = diskPct
			}
			if taskNum, ok := actor.ServerInfo["task_num"].(float64); ok {
				serverInfo.TaskNum = int32(taskNum)
			}
		}

		protoActors[i] = &protocol.ActorInfo{
			IP:         actor.IP,
			InnerIP:    actor.InnerIP,
			Port:       actor.Port,
			MachineID:  machineID,
			Nickname:   actor.Nickname,
			ServerInfo: serverInfo,
		}
	}
	return protoActors
}
