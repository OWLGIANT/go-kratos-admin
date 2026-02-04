package handler

import (
	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorListHandler handles actor list requests
type ActorListHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper
}

// NewActorListHandler creates a new actor list handler
func NewActorListHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorListHandler {
	return &ActorListHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_list")),
	}
}

// Handle processes actor list requests
func (h *ActorListHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	h.log.Infof("Received actor.list request: client=%s, isActor=%v", client.ID, client.IsActor)

	// Get all actors
	actors := h.registry.GetAll()
	h.log.Infof("Actor registry has %d actors", len(actors))

	// Convert to response format
	actorList := make([]map[string]interface{}, 0, len(actors))
	for _, actor := range actors {
		actorData := map[string]interface{}{
			"clientId":      actor.ClientID,
			"robotId":       actor.RobotID,
			"exchange":      actor.Exchange,
			"version":       actor.Version,
			"tenantId":      actor.TenantID,
			"status":        actor.Status,
			"balance":       actor.Balance,
			"registeredAt":  actor.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"),
			"lastHeartbeat": actor.LastHeartbeat.Format("2006-01-02T15:04:05Z07:00"),
		}

		// Add server info if available
		if actor.ServerInfo != nil {
			actorData["serverInfo"] = actor.ServerInfo
		}
		if actor.IP != "" {
			actorData["ip"] = actor.IP
		}
		if actor.InnerIP != "" {
			actorData["innerIp"] = actor.InnerIP
		}
		if actor.Port != "" {
			actorData["port"] = actor.Port
		}
		if actor.MachineID != "" {
			actorData["machineId"] = actor.MachineID
		}
		if actor.Nickname != "" {
			actorData["nickname"] = actor.Nickname
		}

		actorList = append(actorList, actorData)
	}

	h.log.Infof("Actor list requested: client=%s, count=%d", client.ID, len(actorList))

	// Send response
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"actors": actorList,
	})
	return client.SendResponse(resp, msg.Action)
}

// BroadcastActorList broadcasts the actor list to all connected clients
func (h *ActorListHandler) BroadcastActorList() {
	actors := h.registry.GetAll()

	actorList := make([]map[string]interface{}, 0, len(actors))
	for _, actor := range actors {
		actorData := map[string]interface{}{
			"clientId":      actor.ClientID,
			"robotId":       actor.RobotID,
			"exchange":      actor.Exchange,
			"version":       actor.Version,
			"tenantId":      actor.TenantID,
			"status":        actor.Status,
			"balance":       actor.Balance,
			"registeredAt":  actor.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"),
			"lastHeartbeat": actor.LastHeartbeat.Format("2006-01-02T15:04:05Z07:00"),
		}

		if actor.ServerInfo != nil {
			actorData["serverInfo"] = actor.ServerInfo
		}
		if actor.IP != "" {
			actorData["ip"] = actor.IP
		}
		if actor.InnerIP != "" {
			actorData["innerIp"] = actor.InnerIP
		}
		if actor.Port != "" {
			actorData["port"] = actor.Port
		}
		if actor.MachineID != "" {
			actorData["machineId"] = actor.MachineID
		}
		if actor.Nickname != "" {
			actorData["nickname"] = actor.Nickname
		}

		actorList = append(actorList, actorData)
	}

	// Create broadcast message
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"actors": actorList,
	})

	adapter := protocol.NewAdapter()
	data, err := adapter.ConvertResponse(resp, protocol.ProtocolTypeLegacy, "actor.sync")
	if err != nil {
		h.log.Errorf("Failed to convert actor list response: %v", err)
		return
	}

	h.manager.Broadcast(data)
	h.log.Infof("Actor list broadcasted: count=%d", len(actorList))
}
