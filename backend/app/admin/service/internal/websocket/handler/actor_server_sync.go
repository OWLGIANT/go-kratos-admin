package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorServerSyncHandler handles actor server data sync messages
type ActorServerSyncHandler struct {
	registry        *ActorRegistry
	manager         *websocket.Manager
	actorListHandler *ActorListHandler
	log             *log.Helper
}

// NewActorServerSyncHandler creates a new actor server sync handler
func NewActorServerSyncHandler(registry *ActorRegistry, manager *websocket.Manager, actorListHandler *ActorListHandler, logger log.Logger) *ActorServerSyncHandler {
	return &ActorServerSyncHandler{
		registry:        registry,
		manager:         manager,
		actorListHandler: actorListHandler,
		log:             log.NewHelper(log.With(logger, "module", "websocket/handler/actor_server_sync")),
	}
}

// Handle processes actor server sync messages
// This is called when an actor sends its server data after connecting
func (h *ActorServerSyncHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	h.log.Infof("Received actor.server_sync: client=%s, isActor=%v, data=%v", client.ID, client.IsActor, msg.Data)

	robotID, _ := msg.Data["robot_id"].(string)
	if robotID == "" {
		// Try to get robot ID from client
		if client.IsActor && client.RobotID != "" {
			robotID = client.RobotID
		} else {
			h.log.Error("Missing robot_id in server sync")
			resp := protocol.NewErrorResponse(400, "Missing robot_id")
			return client.SendResponse(resp, msg.Action)
		}
	}

	// Extract server info
	var serverInfo *ServerStatusInfo
	if si, ok := msg.Data["server_info"].(map[string]any); ok {
		serverInfo = &ServerStatusInfo{}
		if cpu, ok := si["cpu"].(string); ok {
			serverInfo.CPU = cpu
		}
		if ipPool, ok := si["ip_pool"].(float64); ok {
			serverInfo.IPPool = ipPool
		}
		if mem, ok := si["mem"].(float64); ok {
			serverInfo.Mem = mem
		}
		if memPct, ok := si["mem_pct"].(string); ok {
			serverInfo.MemPct = memPct
		}
		if diskPct, ok := si["disk_pct"].(string); ok {
			serverInfo.DiskPct = diskPct
		}
		if taskNum, ok := si["task_num"].(float64); ok {
			serverInfo.TaskNum = int32(taskNum)
		}
		if straVersion, ok := si["stra_version"].(bool); ok {
			serverInfo.StraVersion = straVersion
		}
		if straVersionDetail, ok := si["stra_version_detail"].(map[string]any); ok {
			serverInfo.StraVersionDetail = make(map[string]string)
			for k, v := range straVersionDetail {
				if vs, ok := v.(string); ok {
					serverInfo.StraVersionDetail[k] = vs
				}
			}
		}
		if awsAcct, ok := si["aws_acct"].(string); ok {
			serverInfo.AwsAcct = awsAcct
		}
		if awsZone, ok := si["aws_zone"].(string); ok {
			serverInfo.AwsZone = awsZone
		}
	}

	// Extract other server fields
	ip, _ := msg.Data["ip"].(string)
	innerIP, _ := msg.Data["inner_ip"].(string)
	port, _ := msg.Data["port"].(string)
	machineID, _ := msg.Data["machine_id"].(string)
	nickname, _ := msg.Data["nickname"].(string)

	// Update actor's server info
	if !h.registry.UpdateServerInfo(robotID, serverInfo, ip, innerIP, port, machineID, nickname) {
		h.log.Warnf("Actor not found for server sync: robot_id=%s", robotID)
		resp := protocol.NewErrorResponse(404, "Actor not found")
		return client.SendResponse(resp, msg.Action)
	}

	h.log.Infof("Actor server info synced: robot_id=%s, ip=%s, machine_id=%s", robotID, ip, machineID)

	// Broadcast updated actor list to all clients
	if h.actorListHandler != nil {
		h.actorListHandler.BroadcastActorList()
	}

	// Send success response
	resp := protocol.NewSuccessResponse(map[string]any{
		"robot_id": robotID,
		"synced":   true,
	})
	return client.SendResponse(resp, msg.Action)
}
