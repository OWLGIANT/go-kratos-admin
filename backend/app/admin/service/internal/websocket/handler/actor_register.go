package handler

import (
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// ActorInfo Actor 信息
type ActorInfo struct {
	ClientID      string                    `json:"client_id"`
	RobotID       string                    `json:"robot_id"`
	Exchange      string                    `json:"exchange"`
	Version       string                    `json:"version"`
	TenantID      uint32                    `json:"tenant_id"`
	Status        string                    `json:"status"`
	Balance       float64                   `json:"balance"`
	RegisteredAt  time.Time                 `json:"registered_at"`
	LastHeartbeat time.Time                 `json:"last_heartbeat"`
	ServerInfo    *protocol.ServerStatusInfo `json:"server_info,omitempty"`
	IP            string                    `json:"ip,omitempty"`
	InnerIP       string                    `json:"inner_ip,omitempty"`
	Port          string                    `json:"port,omitempty"`
	MachineID     string                    `json:"machine_id,omitempty"`
	Nickname      string                    `json:"nickname,omitempty"`
}

// ActorRegistry Actor 注册表
type ActorRegistry struct {
	// actors 映射 robot_id 到 ActorInfo
	actors map[string]*ActorInfo
	// clientToRobot 映射 client_id 到 robot_id
	clientToRobot map[string]string
	mu            sync.RWMutex
}

// NewActorRegistry 创建新的 Actor 注册表
func NewActorRegistry() *ActorRegistry {
	return &ActorRegistry{
		actors:        make(map[string]*ActorInfo),
		clientToRobot: make(map[string]string),
	}
}

// Register 注册 Actor
func (r *ActorRegistry) Register(info *ActorInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actors[info.RobotID] = info
	r.clientToRobot[info.ClientID] = info.RobotID
}

// UnregisterByClientID 通过客户端 ID 注销 Actor
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

// UnregisterByRobotID 通过机器人 ID 注销 Actor
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

// Get 通过机器人 ID 获取 Actor 信息
func (r *ActorRegistry) Get(robotID string) *ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actors[robotID]
}

// GetByClientID 通过客户端 ID 获取 Actor 信息
func (r *ActorRegistry) GetByClientID(clientID string) *ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	robotID, ok := r.clientToRobot[clientID]
	if !ok {
		return nil
	}
	return r.actors[robotID]
}

// GetAll 获取所有注册的 Actor
func (r *ActorRegistry) GetAll() []*ActorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ActorInfo, 0, len(r.actors))
	for _, info := range r.actors {
		result = append(result, info)
	}
	return result
}

// GetByTenant 获取租户的所有 Actor
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

// UpdateStatus 更新 Actor 状态
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

// UpdateHeartbeat 更新 Actor 心跳时间
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

// UpdateServerInfo 更新 Actor 服务器信息
func (r *ActorRegistry) UpdateServerInfo(robotID string, serverInfo *protocol.ServerStatusInfo, ip, innerIP, port, machineID, nickname string) bool {
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

// ActorRegisterHandler Actor 注册处理器
type ActorRegisterHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper
}

// NewActorRegisterHandler 创建新的 Actor 注册处理器
func NewActorRegisterHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorRegisterHandler {
	return &ActorRegisterHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_register")),
	}
}

// Handle 处理 Actor 注册命令
func (h *ActorRegisterHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	h.log.Infof("Received actor.register: client=%s, isActor=%v", client.ID, client.IsActor)

	// 获取 payload
	payload, ok := cmd.Payload.(*protocol.ActorRegisterCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in registration")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request
	if req.RobotID == "" {
		h.log.Error("Missing robot_id in registration")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Missing robot_id")
	}

	// 使用客户端的租户 ID（如果消息中未提供）
	tenantID := req.TenantID
	if tenantID == 0 {
		tenantID = client.TenantID
	}

	// 创建 Actor 信息
	info := &ActorInfo{
		ClientID:      client.ID,
		RobotID:       req.RobotID,
		Exchange:      req.Exchange,
		Version:       req.Version,
		TenantID:      tenantID,
		Status:        "connected",
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
		IP:            req.IP,
		InnerIP:       req.InnerIP,
		Port:          req.Port,
		MachineID:     req.MachineID,
		Nickname:      req.Nickname,
	}

	// 注册 Actor
	h.registry.Register(info)

	// 设置客户端为 Actor
	client.SetActorInfo(req.RobotID)

	h.log.Infof("Actor registered: robot_id=%s, exchange=%s, version=%s, tenant_id=%d, client_id=%s",
		req.RobotID, req.Exchange, req.Version, tenantID, client.ID)

	// 发送成功响应
	respPayload := &protocol.ActorRegisterCmd{
		Response: &protocol.ActorRegisterResponse{
			Registered: true,
			ClientID:   client.ID,
		},
	}
	return client.SendResponse(protocol.CommandTypeActorRegister, cmd.RequestID, cmd.Seq, respPayload)
}

// HandleUnregister 处理 Actor 注销
func (h *ActorRegisterHandler) HandleUnregister(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.ActorUnregisterCmd)

	var robotID string
	if ok && payload.Request != nil {
		robotID = payload.Request.RobotID
	}

	var info *ActorInfo
	if robotID != "" {
		info = h.registry.UnregisterByRobotID(robotID)
	} else {
		info = h.registry.UnregisterByClientID(client.ID)
	}

	if info != nil {
		h.log.Infof("Actor unregistered: robot_id=%s, client_id=%s", info.RobotID, info.ClientID)
	}

	respPayload := &protocol.ActorUnregisterCmd{
		Response: &protocol.ActorUnregisterResponse{
			Success: true,
		},
	}
	return client.SendResponse(protocol.CommandTypeActorUnregister, cmd.RequestID, cmd.Seq, respPayload)
}

// OnClientDisconnect 客户端断开连接时调用
func (h *ActorRegisterHandler) OnClientDisconnect(clientID string) {
	info := h.registry.UnregisterByClientID(clientID)
	if info != nil {
		h.log.Infof("Actor disconnected: robot_id=%s, client_id=%s", info.RobotID, clientID)
	}
}
