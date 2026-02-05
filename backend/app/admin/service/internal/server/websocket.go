package server

import (
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	authnEngine "github.com/tx7do/kratos-authn/engine"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data"
	"go-wind-admin/app/admin/service/internal/service"
	ws "go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/handler"
	"go-wind-admin/app/admin/service/internal/websocket/middleware"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 暂时允许所有来源
	},
}

// WebSocketServer WebSocket 服务器
type WebSocketServer struct {
	server     *http.Server
	manager    *ws.Manager
	router     *handler.Router
	authMw     *middleware.AuthMiddleware
	recoveryMw *middleware.RecoveryMiddleware
	log        *log.Helper

	// Actor 管理
	actorRegistry        *handler.ActorRegistry
	actorRegisterHandler *handler.ActorRegisterHandler
	actorCommandHandler  *handler.ActorCommandHandler
	actorListHandler     *handler.ActorListHandler

	// 配置
	readTimeout    time.Duration
	writeTimeout   time.Duration
	pingInterval   time.Duration
	maxMessageSize int64
}

// NewWebSocketServer 创建新的 WebSocket 服务器
func NewWebSocketServer(
	ctx *bootstrap.Context,
	authenticator authnEngine.Authenticator,
	robotSyncService *service.RobotSyncService,
	serverRepo data.ServerRepo,
) *WebSocketServer {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Websocket == nil {
		return nil
	}

	wsCfg := cfg.Server.Websocket
	logger := log.DefaultLogger

	// 创建管理器
	maxConnections := 10000
	maxConnectionsPerUser := 10

	manager := ws.NewManager(
		logger,
		maxConnections,
		maxConnectionsPerUser,
	)

	// 创建路由器
	router := handler.NewRouter(logger)

	// 创建中间件
	authMw := middleware.NewAuthMiddleware(authenticator, logger)
	recoveryMw := middleware.NewRecoveryMiddleware(logger)

	// 注册处理器

	// 心跳处理器
	heartbeatHandler := handler.NewHeartbeatHandler(logger)
	router.Register(protocol.CommandTypeEcho, recoveryMw.Recover(heartbeatHandler.Handle))

	// 机器人同步处理器
	robotSyncHandler := handler.NewRobotSyncHandler(robotSyncService, manager, logger)
	router.Register(protocol.CommandTypeRobotSync, recoveryMw.Recover(robotSyncHandler.Handle))

	// 告警处理器
	alertHandler := handler.NewAlertHandler(manager, logger)
	router.Register(protocol.CommandTypeAlertSend, recoveryMw.Recover(alertHandler.Handle))

	// 踢出用户处理器
	kickHandler := handler.NewKickHandler(manager, logger)
	router.Register(protocol.CommandTypeUserKick, recoveryMw.Recover(kickHandler.Handle))

	// Actor 处理器
	actorRegistry := handler.NewActorRegistry()
	actorRegisterHandler := handler.NewActorRegisterHandler(actorRegistry, manager, logger)
	router.Register(protocol.CommandTypeActorRegister, recoveryMw.Recover(actorRegisterHandler.Handle))
	router.Register(protocol.CommandTypeActorUnregister, recoveryMw.Recover(actorRegisterHandler.HandleUnregister))

	actorStatusHandler := handler.NewActorStatusHandler(actorRegistry, logger)
	router.Register(protocol.CommandTypeActorStatus, recoveryMw.Recover(actorStatusHandler.Handle))
	router.Register(protocol.CommandTypeActorHeartbeat, recoveryMw.Recover(actorStatusHandler.HandleHeartbeat))

	actorCommandHandler := handler.NewActorCommandHandler(actorRegistry, manager, logger)
	router.Register(protocol.CommandTypeRobotResult, recoveryMw.Recover(actorCommandHandler.Handle))

	// Actor 列表处理器
	actorListHandler := handler.NewActorListHandler(actorRegistry, manager, logger)
	router.Register(protocol.CommandTypeActorList, recoveryMw.Recover(actorListHandler.Handle))

	// Actor 服务器同步处理器
	actorServerSyncHandler := handler.NewActorServerSyncHandler(actorRegistry, manager, actorListHandler, serverRepo, logger)
	router.Register(protocol.CommandTypeServerSync, recoveryMw.Recover(actorServerSyncHandler.Handle))

	// 设置命令处理器
	manager.SetCommandHandler(router)

	// 创建 HTTP 服务器
	mux := http.NewServeMux()

	wsServer := &WebSocketServer{
		manager:              manager,
		router:               router,
		authMw:               authMw,
		recoveryMw:           recoveryMw,
		log:                  log.NewHelper(log.With(logger, "module", "websocket-server")),
		actorRegistry:        actorRegistry,
		actorRegisterHandler: actorRegisterHandler,
		actorCommandHandler:  actorCommandHandler,
		actorListHandler:     actorListHandler,
		readTimeout:          60 * time.Second,
		writeTimeout:         10 * time.Second,
		pingInterval:         54 * time.Second,
		maxMessageSize:       512 * 1024,
	}

	mux.HandleFunc(wsCfg.Path, wsServer.handleWebSocket)

	wsServer.server = &http.Server{
		Addr:    wsCfg.Addr,
		Handler: mux,
	}

	wsServer.log.Infof("WebSocket server initialized on %s%s", wsCfg.Addr, wsCfg.Path)

	return wsServer
}

// handleWebSocket 处理 WebSocket 连接
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("Failed to upgrade connection: %v", err)
		return
	}

	// 创建客户端
	client := ws.NewClient(conn, s.manager)

	// 认证客户端
	if err := s.authMw.AuthenticateClient(client, r); err != nil {
		s.log.Errorf("Authentication failed: %v", err)
		conn.Close()
		return
	}

	// 注册客户端
	if err := s.manager.Register(client); err != nil {
		s.log.Errorf("Failed to register client: %v", err)
		conn.Close()
		return
	}

	// 启动客户端读写泵
	go client.WritePump(s.writeTimeout, s.pingInterval)
	go client.ReadPump(s.readTimeout, s.maxMessageSize)

	s.log.Infof("WebSocket connection established: client=%s, user=%s", client.ID, client.Username)
}

// Start 启动 WebSocket 服务器
func (s *WebSocketServer) Start() error {
	s.log.Infof("Starting WebSocket server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Errorf("WebSocket server error: %v", err)
		return err
	}
	return nil
}

// Stop 停止 WebSocket 服务器
func (s *WebSocketServer) Stop() error {
	s.log.Info("Stopping WebSocket server")
	return s.server.Close()
}

// GetActorRegistry 获取 Actor 注册表
func (s *WebSocketServer) GetActorRegistry() *handler.ActorRegistry {
	return s.actorRegistry
}

// GetActorCommandHandler 获取 Actor 命令处理器
func (s *WebSocketServer) GetActorCommandHandler() *handler.ActorCommandHandler {
	return s.actorCommandHandler
}

// SendActorCommand 发送命令给 Actor
func (s *WebSocketServer) SendActorCommand(robotID, action string, data map[string]interface{}) (*handler.CommandResultData, error) {
	return s.actorCommandHandler.SendCommand(robotID, action, data)
}

// GetConnectedActors 获取所有连接的 Actor
func (s *WebSocketServer) GetConnectedActors() []*handler.ActorInfo {
	return s.actorRegistry.GetAll()
}

// GetActorsByTenant 获取租户的所有 Actor
func (s *WebSocketServer) GetActorsByTenant(tenantID uint32) []*handler.ActorInfo {
	return s.actorRegistry.GetByTenant(tenantID)
}
