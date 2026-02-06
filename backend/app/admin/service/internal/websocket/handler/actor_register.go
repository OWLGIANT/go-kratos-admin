package handler

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/data"
	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"

	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

// ActorServerInfo Actor 信息（对应数据库 Server 表）
type ActorServerInfo struct {
	ent.Server
}

// ActorRegistry Actor 注册表
type ActorRegistry struct {
	// actorServers 映射 IP 到 ActorServerInfo
	actorServers map[string]*ActorServerInfo
	// clientToIP 映射 client_id 到 IP
	clientToIP map[string]string
	mu         sync.RWMutex
}

// NewActorRegistry 创建新的 Actor 注册表
func NewActorRegistry() *ActorRegistry {
	return &ActorRegistry{
		actorServers: make(map[string]*ActorServerInfo),
		clientToIP:   make(map[string]string),
	}
}

// Register 注册 Actor
func (r *ActorRegistry) Register(info *ActorServerInfo, clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actorServers[info.IP] = info
	r.clientToIP[clientID] = info.IP
}

// UnregisterByClientID 通过客户端 ID 注销 Actor
func (r *ActorRegistry) UnregisterByClientID(clientID string) *ActorServerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	ip, ok := r.clientToIP[clientID]
	if !ok {
		return nil
	}

	info := r.actorServers[ip]
	delete(r.actorServers, ip)
	delete(r.clientToIP, clientID)
	return info
}

// UnregisterByIP 通过 IP 注销 Actor
func (r *ActorRegistry) UnregisterByIP(ip string) *ActorServerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actorServers[ip]
	if !ok {
		return nil
	}

	delete(r.actorServers, ip)
	// 需要遍历找到对应的 clientID
	for cid, cip := range r.clientToIP {
		if cip == ip {
			delete(r.clientToIP, cid)
			break
		}
	}
	return info
}

// Get 通过 IP 获取 Actor 信息
func (r *ActorRegistry) Get(ip string) *ActorServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actorServers[ip]
}

// GetByClientID 通过客户端 ID 获取 Actor 信息
func (r *ActorRegistry) GetByClientID(clientID string) *ActorServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ip, ok := r.clientToIP[clientID]
	if !ok {
		return nil
	}
	return r.actorServers[ip]
}

// GetAll 获取所有注册的 Actor
func (r *ActorRegistry) GetAll() []*ActorServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ActorServerInfo, 0, len(r.actorServers))
	for _, info := range r.actorServers {
		result = append(result, info)
	}
	return result
}

// UpdateServerInfo 更新 Actor 服务器信息
func (r *ActorRegistry) UpdateServerInfo(ip string, serverInfo map[string]interface{}, innerIP, port, machineID, nickname string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.actorServers[ip]
	if !ok {
		return false
	}

	if serverInfo != nil {
		info.ServerInfo = serverInfo
	}
	if innerIP != "" {
		info.InnerIP = innerIP
	}
	if port != "" {
		info.Port = port
	}
	if machineID != "" {
		info.MachineID = &machineID
	}
	if nickname != "" {
		info.Nickname = nickname
	}
	return true
}

// GetClientIDByIP 通过 IP 获取客户端 ID
func (r *ActorRegistry) GetClientIDByIP(ip string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for cid, cip := range r.clientToIP {
		if cip == ip {
			return cid
		}
	}
	return ""
}

// ActorRegisterHandler Actor 注册处理器
type ActorRegisterHandler struct {
	registry   *ActorRegistry
	manager    *websocket.Manager
	serverRepo data.ServerRepo
	log        *log.Helper
}

// NewActorRegisterHandler 创建新的 Actor 注册处理器
func NewActorRegisterHandler(registry *ActorRegistry, manager *websocket.Manager, serverRepo data.ServerRepo, logger log.Logger) *ActorRegisterHandler {
	return &ActorRegisterHandler{
		registry:   registry,
		manager:    manager,
		serverRepo: serverRepo,
		log:        log.NewHelper(log.With(logger, "module", "websocket/handler/actor_register")),
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
	if req.IP == "" {
		h.log.Error("Missing ip in registration")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Missing ip")
	}

	// 转换 ServerStatusInfo 为 map
	var serverInfoMap map[string]interface{}
	// 可以根据需要从 req 中提取服务器信息

	// 插入或更新到数据库
	ctx := context.Background()
	upsertReq := &tradingV1.UpsertServerByIPRequest{
		Ip:        req.IP,
		InnerIp:   req.InnerIP,
		Port:      req.Port,
		Nickname:  req.Nickname,
		MachineId: req.MachineID,
	}

	dbServer, err := h.serverRepo.UpsertByIP(ctx, upsertReq)
	if err != nil {
		h.log.Errorf("Failed to upsert server to database: %v", err)
		return client.SendError(cmd.RequestID, cmd.Seq, 500, "Database error")
	}

	// 创建 Actor 信息
	now := time.Now()
	info := &ActorServerInfo{}
	info.ID = dbServer.Id
	info.IP = req.IP
	info.InnerIP = req.InnerIP
	info.Port = req.Port
	info.Nickname = req.Nickname
	if req.MachineID != "" {
		info.MachineID = &req.MachineID
	}
	info.ServerInfo = serverInfoMap
	info.CreatedAt = &now
	info.UpdatedAt = &now

	// 注册 Actor
	h.registry.Register(info, client.ID)

	// 设置客户端为 Actor
	client.SetActorInfo(req.IP)

	h.log.Infof("Actor registered: ip=%s, client_id=%s", req.IP, client.ID)

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
	info := h.registry.UnregisterByClientID(client.ID)

	if info != nil {
		h.log.Infof("Actor unregistered: ip=%s, client_id=%s", info.IP, client.ID)
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
		h.log.Infof("Actor disconnected: ip=%s, client_id=%s", info.IP, clientID)
	}
}
