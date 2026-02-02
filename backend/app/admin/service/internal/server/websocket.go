package server

import (
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	authnEngine "github.com/tx7do/kratos-authn/engine"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/service"
	ws "go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/handler"
	"go-wind-admin/app/admin/service/internal/websocket/middleware"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// WebSocketServer wraps the WebSocket server
type WebSocketServer struct {
	server      *http.Server
	manager     *ws.Manager
	router      *handler.Router
	authMw      *middleware.AuthMiddleware
	recoveryMw  *middleware.RecoveryMiddleware
	log         *log.Helper

	// Configuration
	readTimeout   time.Duration
	writeTimeout  time.Duration
	pingInterval  time.Duration
	maxMessageSize int64
}

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer(
	ctx *bootstrap.Context,
	authenticator authnEngine.Authenticator,
	robotSyncService *service.RobotSyncService,
) *WebSocketServer {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Websocket == nil {
		return nil
	}

	wsCfg := cfg.Server.Websocket
	logger := log.DefaultLogger

	// Create manager with default limits
	maxConnections := 10000
	maxConnectionsPerUser := 10

	manager := ws.NewManager(
		logger,
		maxConnections,
		maxConnectionsPerUser,
	)

	// Create router
	router := handler.NewRouter(logger)

	// Create middlewares
	authMw := middleware.NewAuthMiddleware(authenticator, logger)
	recoveryMw := middleware.NewRecoveryMiddleware(logger)

	// Register handlers
	heartbeatHandler := handler.NewHeartbeatHandler(logger)
	router.Register("heartbeat", recoveryMw.Recover(heartbeatHandler.Handle))

	robotSyncHandler := handler.NewRobotSyncHandler(robotSyncService, manager, logger)
	router.Register("robot.sync", recoveryMw.Recover(robotSyncHandler.Handle))

	alertHandler := handler.NewAlertHandler(manager, logger)
	router.Register("alert.send", recoveryMw.Recover(alertHandler.Handle))

	kickHandler := handler.NewKickHandler(manager, logger)
	router.Register("auth.kick", recoveryMw.Recover(kickHandler.Handle))

	// Set message handler
	manager.SetMessageHandler(router)

	// Create HTTP server
	mux := http.NewServeMux()

	wsServer := &WebSocketServer{
		manager:        manager,
		router:         router,
		authMw:         authMw,
		recoveryMw:     recoveryMw,
		log:            log.NewHelper(log.With(logger, "module", "websocket-server")),
		readTimeout:    60 * time.Second,  // Default pong wait
		writeTimeout:   10 * time.Second,  // Default write wait
		pingInterval:   54 * time.Second,  // Default ping interval
		maxMessageSize: 512 * 1024,        // Default 512KB
	}

	mux.HandleFunc(wsCfg.Path, wsServer.handleWebSocket)

	wsServer.server = &http.Server{
		Addr:    wsCfg.Addr,
		Handler: mux,
	}

	wsServer.log.Infof("WebSocket server initialized on %s%s", wsCfg.Addr, wsCfg.Path)

	return wsServer
}

// handleWebSocket handles WebSocket connections
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("Failed to upgrade connection: %v", err)
		return
	}

	// Create client
	client := ws.NewClient(conn, s.manager)

	// Authenticate client
	if err := s.authMw.AuthenticateClient(client, r); err != nil {
		s.log.Errorf("Authentication failed: %v", err)
		conn.Close()
		return
	}

	// Register client
	if err := s.manager.Register(client); err != nil {
		s.log.Errorf("Failed to register client: %v", err)
		conn.Close()
		return
	}

	// Start client pumps
	go client.WritePump(s.writeTimeout, s.pingInterval)
	go client.ReadPump(s.readTimeout, s.maxMessageSize)

	s.log.Infof("WebSocket connection established: client=%s, user=%s", client.ID, client.Username)
}

// Start starts the WebSocket server
func (s *WebSocketServer) Start() error {
	s.log.Infof("Starting WebSocket server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Errorf("WebSocket server error: %v", err)
		return err
	}
	return nil
}

// Stop stops the WebSocket server
func (s *WebSocketServer) Stop() error {
	s.log.Info("Stopping WebSocket server")
	return s.server.Close()
}
