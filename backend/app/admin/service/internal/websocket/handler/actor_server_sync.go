package handler

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/data"
	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"

	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

// ActorServerSyncHandler Actor 服务器同步处理器
type ActorServerSyncHandler struct {
	registry         *ActorRegistry
	manager          *websocket.Manager
	actorListHandler *ActorListHandler
	serverRepo       data.ServerRepo
	log              *log.Helper
}

// NewActorServerSyncHandler 创建新的 Actor 服务器同步处理器
func NewActorServerSyncHandler(registry *ActorRegistry, manager *websocket.Manager, actorListHandler *ActorListHandler, serverRepo data.ServerRepo, logger log.Logger) *ActorServerSyncHandler {
	return &ActorServerSyncHandler{
		registry:         registry,
		manager:          manager,
		actorListHandler: actorListHandler,
		serverRepo:       serverRepo,
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
	if req.IP == "" {
		h.log.Error("Missing ip in server sync")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Missing ip")
	}

	// 转换 ServerStatusInfo 为 map
	var serverInfoMap map[string]interface{}
	if req.ServerInfo != nil {
		serverInfoMap = map[string]interface{}{
			"cpu":      req.ServerInfo.CPU,
			"ip_pool":  req.ServerInfo.IPPool,
			"mem":      req.ServerInfo.Mem,
			"mem_pct":  req.ServerInfo.MemPct,
			"disk_pct": req.ServerInfo.DiskPct,
			"task_num": req.ServerInfo.TaskNum,
		}
	}

	// 更新 Actor 的服务器信息
	h.registry.UpdateServerInfo(req.IP, serverInfoMap, req.InnerIP, req.Port, req.MachineID, req.Nickname)

	h.log.Infof("Actor server info synced: ip=%s, machine_id=%s", req.IP, req.MachineID)

	// 插入或更新服务器到数据库
	var serverInfo *tradingV1.ServerStatusInfo
	if req.ServerInfo != nil {
		serverInfo = &tradingV1.ServerStatusInfo{
			Cpu:     req.ServerInfo.CPU,
			IpPool:  req.ServerInfo.IPPool,
			Mem:     req.ServerInfo.Mem,
			MemPct:  req.ServerInfo.MemPct,
			DiskPct: req.ServerInfo.DiskPct,
			TaskNum: req.ServerInfo.TaskNum,
		}
	}

	upsertReq := &tradingV1.UpsertServerByIPRequest{
		Ip:         req.IP,
		InnerIp:    req.InnerIP,
		Port:       req.Port,
		Nickname:   req.Nickname,
		MachineId:  req.MachineID,
		ServerInfo: serverInfo,
	}

	ctx := context.Background()
	_, err := h.serverRepo.UpsertByIP(ctx, upsertReq)
	if err != nil {
		h.log.Errorf("Failed to upsert server to database: %v", err)
	} else {
		h.log.Infof("Server upserted to database: ip=%s", req.IP)
	}

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
