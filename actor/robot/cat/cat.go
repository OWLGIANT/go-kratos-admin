package cat

import (
	"actor/helper"
	"actor/robot/cat/config"
	"actor/robot/cat/quant"
	"actor/server/backend"
	"actor/third/log"
	"context"
	"fmt"
	"sync"
)

// Robot Cat 机器人
type Robot struct {
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	quant     *quant.Quant
	robotID   string
	status    string
	logger    log.Logger
	buildTime string

	// backend 客户端用于同步状态
	backendClient *backend.Client
}

// NewRobot 创建新的 Cat 机器人
func NewRobot(ctx context.Context, robotID string, logger log.Logger, buildTime string) *Robot {
	childCtx, cancel := context.WithCancel(ctx)
	return &Robot{
		ctx:       childCtx,
		cancel:    cancel,
		robotID:   robotID,
		status:    "created",
		logger:    logger,
		buildTime: buildTime,
	}
}

// SetBackendClient 设置 backend 客户端
func (r *Robot) SetBackendClient(client *backend.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backendClient = client
}

// Start 启动机器人
func (r *Robot) Start(data map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == "running" {
		return fmt.Errorf("robot already running")
	}

	// 从 data 加载配置
	cfg := config.LoadConfigFromMap(data)
	if cfg.TaskUid == "" {
		cfg.TaskUid = r.robotID
	}

	// 创建 quant 实例
	r.quant = quant.NewQuant(r.ctx, r.buildTime)

	// 初始化
	logFile := fmt.Sprintf("logs/cat_%s.log", r.robotID)
	if err := r.quant.QuantInitConfig(logFile); err != nil {
		r.status = "error"
		r.logger.Errorf("Failed to init quant: %v", err)
		return err
	}

	// 启动交易
	go func() {
		r.quant.Run()
	}()

	r.status = "running"
	r.logger.Infof("Cat robot %s started", r.robotID)

	// 通知 backend
	r.sendStatusUpdate()

	return nil
}

// Stop 停止机器人
func (r *Robot) Stop(data map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.quant != nil {
		r.quant.OnExit(helper.NORMAL_EXIT_MSG)
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.status = "stopped"
	r.logger.Infof("Cat robot %s stopped", r.robotID)

	// 通知 backend
	r.sendStatusUpdate()

	return nil
}

// GetStatus 获取机器人状态
func (r *Robot) GetStatus() (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]interface{}{
		"robot_id": r.robotID,
		"status":   r.status,
		"type":     "cat",
	}

	if r.quant != nil {
		r.quant.StatusLock.Lock()
		result["quant_status"] = r.quant.Status
		result["quant_msg"] = r.quant.Msg
		r.quant.StatusLock.Unlock()
	}

	return result, nil
}

// UpdateConfig 更新配置
func (r *Robot) UpdateConfig(data map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 更新配置逻辑
	r.logger.Infof("Cat robot %s config updated", r.robotID)
	return nil
}

// sendStatusUpdate 发送状态更新到 backend
func (r *Robot) sendStatusUpdate() {
	if r.backendClient != nil && r.backendClient.IsConnected() {
		balance := 0.0
		r.backendClient.SendStatus(r.status, balance)
	}
}

// RobotID 返回机器人ID
func (r *Robot) RobotID() string {
	return r.robotID
}

// Status 返回机器人状态
func (r *Robot) Status() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}
