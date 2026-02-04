package handler

import (
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorInfo stores information about a connected actor
type ActorInfo struct {
	ClientID      string    `json:"client_id"`
	RobotID       string    `json:"robot_id"`
	Exchange      string    `json:"exchange"`
	Version       string    `json:"version"`
	TenantID      uint32    `json:"tenant_id"`
	Status        string    `json:"status"`
	Balance       float64   `json:"balance"`
	RegisteredAt  time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	// Server related info
	ServerInfo *ServerStatusInfo `json:"server_info,omitempty"`
	IP         string            `json:"ip,omitempty"`
	InnerIP    string            `json:"inner_ip,omitempty"`
	Port       string            `json:"port,omitempty"`
	MachineID  string            `json:"machine_id,omitempty"`
	Nickname   string            `json:"nickname,omitempty"`
}

// ServerStatusInfo stores server status information
type ServerStatusInfo struct {
	CPU              string            `json:"cpu,omitempty"`
	IPPool           float64           `json:"ip_pool,omitempty"`
	Mem              float64           `json:"mem,omitempty"`
	MemPct           string            `json:"mem_pct,omitempty"`
	DiskPct          string            `json:"disk_pct,omitempty"`
	TaskNum          int32             `json:"task_num,omitempty"`
	StraVersion      bool              `json:"stra_version,omitempty"`
	StraVersionDetail map[string]string `json:"stra_version_detail,omitempty"`
	AwsAcct          string            `json:"aws_acct,omitempty"`
	AwsZone          string            `json:"aws_zone,omitempty"`
}

// ActorRegistry manages registered actors
type ActorRegistry struct {
	// actors maps robot_id to ActorInfo
	actors map[string]*ActorInfo
	// clientToRobot maps client_id to robot_id
	clientToRobot map[string]string
	mu            sync.RWMutex
}

// NewActorRegistry creates a new actor registry
func NewActorRegistry() *ActorRegistry {
	return &ActorRegistry{
		actors:        make(map[string]*ActorInfo),
		clientToRobot: make(map[string]string),
	}
}

// Register registers an actor
func (r *ActorRegistry) Register(info *ActorInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actors[info.RobotID] = info
	r.clientToRobot[info.ClientID] = info.RobotID
}

// Unregister removes an actor by client ID
func (r *ActorRegistry) UnregisterByClientID(clientID string) *ActorInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	robotID, ok := r.clientToRobot[clientID]
	if !ok {
		return nil
	}

	info := r.actors[robotID]
	delete(r.actors, robotID)
	delete(r.clientToRobot, clientID)
	return info
}

// UnregisterByRobotID removes an actor by robot ID
func (r *ActorRegistry) UnregisterByRobotID(robotID string) *ActorInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actors[robotID]
	if !ok {
		return nil
	}

	delete(r.actors, robotID)
	delete(r.clientToRobot, info.ClientID)
	return info
}

// Get returns actor info by robot ID
func (r *ActorRegistry) Get(robotID string) *ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actors[robotID]
}

// GetByClientID returns actor info by client ID
func (r *ActorRegistry) GetByClientID(clientID string) *ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	robotID, ok := r.clientToRobot[clientID]
	if !ok {
		return nil
	}
	return r.actors[robotID]
}

// GetAll returns all registered actors
func (r *ActorRegistry) GetAll() []*ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ActorInfo, 0, len(r.actors))
	for _, info := range r.actors {
		result = append(result, info)
	}
	return result
}

// GetByTenant returns all actors for a tenant
func (r *ActorRegistry) GetByTenant(tenantID uint32) []*ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ActorInfo
	for _, info := range r.actors {
		if info.TenantID == tenantID {
			result = append(result, info)
		}
	}
	return result
}

// UpdateStatus updates actor status
func (r *ActorRegistry) UpdateStatus(robotID, status string, balance float64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actors[robotID]
	if !ok {
		return false
	}

	info.Status = status
	info.Balance = balance
	return true
}

// UpdateHeartbeat updates actor heartbeat time
func (r *ActorRegistry) UpdateHeartbeat(robotID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actors[robotID]
	if !ok {
		return false
	}

	info.LastHeartbeat = time.Now()
	return true
}

// UpdateServerInfo updates actor server information
func (r *ActorRegistry) UpdateServerInfo(robotID string, serverInfo *ServerStatusInfo, ip, innerIP, port, machineID, nickname string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actors[robotID]
	if !ok {
		return false
	}

	info.ServerInfo = serverInfo
	if ip != "" {
		info.IP = ip
	}
	if innerIP != "" {
		info.InnerIP = innerIP
	}
	if port != "" {
		info.Port = port
	}
	if machineID != "" {
		info.MachineID = machineID
	}
	if nickname != "" {
		info.Nickname = nickname
	}
	return true
}

// ActorRegisterHandler handles actor registration messages
type ActorRegisterHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper
}

// NewActorRegisterHandler creates a new actor register handler
func NewActorRegisterHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorRegisterHandler {
	return &ActorRegisterHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_register")),
	}
}

// Handle processes actor registration messages
func (h *ActorRegisterHandler) Handle(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	h.log.Infof("Received actor.register: client=%s, isActor=%v, data=%v", client.ID, client.IsActor, msg.Data)

	robotID, _ := msg.Data["robot_id"].(string)
	if robotID == "" {
		h.log.Error("Missing robot_id in registration")
		resp := protocol.NewErrorResponse(400, "Missing robot_id")
		return client.SendResponse(resp, msg.Action)
	}

	exchange, _ := msg.Data["exchange"].(string)
	version, _ := msg.Data["version"].(string)
	tenantID := uint32(0)
	if tid, ok := msg.Data["tenant_id"].(float64); ok {
		tenantID = uint32(tid)
	}

	// Use client's tenant ID if not provided in message
	if tenantID == 0 {
		tenantID = client.TenantID
	}

	// Create actor info
	info := &ActorInfo{
		ClientID:      client.ID,
		RobotID:       robotID,
		Exchange:      exchange,
		Version:       version,
		TenantID:      tenantID,
		Status:        "connected",
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	// Register actor
	h.registry.Register(info)

	h.log.Infof("Actor registered: robot_id=%s, exchange=%s, version=%s, tenant_id=%d, client_id=%s",
		robotID, exchange, version, tenantID, client.ID)

	// Send success response
	resp := protocol.NewSuccessResponse(map[string]interface{}{
		"robot_id":   robotID,
		"registered": true,
	})
	return client.SendResponse(resp, msg.Action)
}

// HandleUnregister processes actor unregistration
func (h *ActorRegisterHandler) HandleUnregister(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	robotID, _ := msg.Data["robot_id"].(string)

	var info *ActorInfo
	if robotID != "" {
		info = h.registry.UnregisterByRobotID(robotID)
	} else {
		info = h.registry.UnregisterByClientID(client.ID)
	}

	if info != nil {
		h.log.Infof("Actor unregistered: robot_id=%s, client_id=%s", info.RobotID, info.ClientID)
	}

	resp := protocol.NewSuccessResponse(nil)
	return client.SendResponse(resp, msg.Action)
}

// OnClientDisconnect should be called when a client disconnects
func (h *ActorRegisterHandler) OnClientDisconnect(clientID string) {
	info := h.registry.UnregisterByClientID(clientID)
	if info != nil {
		h.log.Infof("Actor disconnected: robot_id=%s, client_id=%s", info.RobotID, clientID)
	}
}
