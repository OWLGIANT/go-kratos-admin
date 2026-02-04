package handler

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// PendingCommand 等待响应的命令
type PendingCommand struct {
	RequestID  string
	Action     string
	RobotID    string
	SentAt     time.Time
	ResultChan chan *CommandResultData
}

// CommandResultData 命令结果数据
type CommandResultData struct {
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Result    interface{} `json:"result,omitempty"`
}

// ActorCommandHandler Actor 命令处理器
type ActorCommandHandler struct {
	registry *ActorRegistry
	manager  *websocket.Manager
	log      *log.Helper

	// 等待响应的命令
	pending   map[string]*PendingCommand
	pendingMu sync.RWMutex

	// 命令超时时间
	timeout time.Duration
}

// NewActorCommandHandler 创建新的 Actor 命令处理器
func NewActorCommandHandler(registry *ActorRegistry, manager *websocket.Manager, logger log.Logger) *ActorCommandHandler {
	return &ActorCommandHandler{
		registry: registry,
		manager:  manager,
		log:      log.NewHelper(log.With(logger, "module", "websocket/handler/actor_command")),
		pending:  make(map[string]*PendingCommand),
		timeout:  30 * time.Second,
	}
}

// SetTimeout 设置命令超时时间
func (h *ActorCommandHandler) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// SendCommand 发送命令给 Actor 并等待响应
func (h *ActorCommandHandler) SendCommand(robotID, action string, data map[string]interface{}) (*CommandResultData, error) {
	// 获取 Actor 信息
	info := h.registry.Get(robotID)
	if info == nil {
		return &CommandResultData{
			Success: false,
			Error:   "actor not found",
		}, nil
	}

	// 获取客户端
	client, err := h.manager.GetClient(info.ClientID)
	if err != nil {
		return &CommandResultData{
			Success: false,
			Error:   "actor client not connected",
		}, nil
	}

	// 生成请求 ID
	requestID := uuid.New().String()

	// 创建等待命令
	pending := &PendingCommand{
		RequestID:  requestID,
		Action:     action,
		RobotID:    robotID,
		SentAt:     time.Now(),
		ResultChan: make(chan *CommandResultData, 1),
	}

	// 注册等待命令
	h.pendingMu.Lock()
	h.pending[requestID] = pending
	h.pendingMu.Unlock()

	// 退出时清理
	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, requestID)
		h.pendingMu.Unlock()
	}()

	// 构建命令消息
	payload, _ := json.Marshal(data)
	cmd := protocol.NewCommandWithRequestID(protocol.CommandTypeRobotCommand, requestID)
	cmd.Payload = &protocol.RobotCommandCmd{
		Request: &protocol.RobotCommandRequest{
			RobotID:   robotID,
			Action:    action,
			Payload:   payload,
			TimeoutMs: int32(h.timeout.Milliseconds()),
		},
	}

	// 发送命令
	if err := client.SendCommand(cmd); err != nil {
		return &CommandResultData{
			Success: false,
			Error:   "failed to send command",
		}, nil
	}

	h.log.Infof("Command sent: robot_id=%s, action=%s, request_id=%s", robotID, action, requestID)

	// 等待响应或超时
	select {
	case result := <-pending.ResultChan:
		return result, nil
	case <-time.After(h.timeout):
		return &CommandResultData{
			RequestID: requestID,
			Success:   false,
			Error:     "command timeout",
		}, nil
	}
}

// Handle 处理来自 Actor 的命令结果
func (h *ActorCommandHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.RobotResultCmd)
	if !ok || payload.Request == nil {
		h.log.Warn("Invalid payload in command result")
		return nil
	}

	requestID := payload.Request.RequestID
	if requestID == "" {
		h.log.Warn("Missing request_id in command result")
		return nil
	}

	// 获取等待命令
	h.pendingMu.RLock()
	pending, ok := h.pending[requestID]
	h.pendingMu.RUnlock()

	if !ok {
		h.log.Warnf("No pending command found for request_id=%s", requestID)
		return nil
	}

	// 构建结果
	result := &CommandResultData{
		RequestID: requestID,
		Success:   payload.Request.Success,
		Error:     payload.Request.Error,
	}

	if len(payload.Request.Result) > 0 {
		var resultData interface{}
		if err := json.Unmarshal(payload.Request.Result, &resultData); err == nil {
			result.Result = resultData
		}
	}

	h.log.Infof("Command result received: request_id=%s, success=%v", requestID, result.Success)

	// 发送结果给等待的 goroutine
	select {
	case pending.ResultChan <- result:
	default:
		h.log.Warnf("Result channel full for request_id=%s", requestID)
	}

	return nil
}

// SendStartCommand 发送启动命令给 Robot
func (h *ActorCommandHandler) SendStartCommand(robotID string, config map[string]interface{}) (*CommandResultData, error) {
	return h.SendCommand(robotID, "robot.start", config)
}

// SendStopCommand 发送停止命令给 Robot
func (h *ActorCommandHandler) SendStopCommand(robotID string, graceful bool, reason string) (*CommandResultData, error) {
	return h.SendCommand(robotID, "robot.stop", map[string]interface{}{
		"graceful": graceful,
		"reason":   reason,
	})
}

// SendStatusCommand 发送状态查询命令给 Robot
func (h *ActorCommandHandler) SendStatusCommand(robotID string) (*CommandResultData, error) {
	return h.SendCommand(robotID, "robot.status", nil)
}

// SendConfigCommand 发送配置更新命令给 Robot
func (h *ActorCommandHandler) SendConfigCommand(robotID string, config map[string]interface{}) (*CommandResultData, error) {
	return h.SendCommand(robotID, "robot.config", config)
}

// GetPendingCommands 获取所有等待的命令
func (h *ActorCommandHandler) GetPendingCommands() []*PendingCommand {
	h.pendingMu.RLock()
	defer h.pendingMu.RUnlock()

	result := make([]*PendingCommand, 0, len(h.pending))
	for _, cmd := range h.pending {
		result = append(result, cmd)
	}
	return result
}

// CleanupExpiredCommands 清理过期的等待命令
func (h *ActorCommandHandler) CleanupExpiredCommands() {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	now := time.Now()
	for id, cmd := range h.pending {
		if now.Sub(cmd.SentAt) > h.timeout {
			// 发送超时结果
			select {
			case cmd.ResultChan <- &CommandResultData{
				RequestID: cmd.RequestID,
				Success:   false,
				Error:     "command timeout (cleanup)",
			}:
			default:
			}
			delete(h.pending, id)
		}
	}
}
