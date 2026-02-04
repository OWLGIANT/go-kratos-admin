package service

import (
	"context"
	"fmt"
	"sync"

	"actor/robot/cat"
	"actor/server/backend"
	"actor/third/log"
)

// RobotManager 机器人管理器
type RobotManager struct {
	mu            sync.RWMutex
	robots        map[string]*cat.Robot
	logger        log.Logger
	buildTime     string
	ctx           context.Context
	backendClient *backend.Client
}

// NewRobotManager 创建机器人管理器
func NewRobotManager(ctx context.Context, logger log.Logger, buildTime string) *RobotManager {
	return &RobotManager{
		robots:    make(map[string]*cat.Robot),
		logger:    logger,
		buildTime: buildTime,
		ctx:       ctx,
	}
}

// SetBackendClient 设置 backend 客户端
func (m *RobotManager) SetBackendClient(client *backend.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backendClient = client
}

// CreateRobot 创建机器人
func (m *RobotManager) CreateRobot(robotID string, robotType string, data map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.robots[robotID]; exists {
		return fmt.Errorf("robot %s already exists", robotID)
	}

	switch robotType {
	case "cat":
		robot := cat.NewRobot(m.ctx, robotID, m.logger, m.buildTime)
		if m.backendClient != nil {
			robot.SetBackendClient(m.backendClient)
		}
		m.robots[robotID] = robot
		m.logger.Infof("Created cat robot: %s", robotID)
		return nil
	default:
		return fmt.Errorf("unsupported robot type: %s", robotType)
	}
}

// StartRobot 启动机器人
func (m *RobotManager) StartRobot(robotID string, data map[string]interface{}) error {
	m.mu.RLock()
	robot, exists := m.robots[robotID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("robot %s not found", robotID)
	}

	return robot.Start(data)
}

// StopRobot 停止机器人
func (m *RobotManager) StopRobot(robotID string, data map[string]interface{}) error {
	m.mu.RLock()
	robot, exists := m.robots[robotID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("robot %s not found", robotID)
	}

	return robot.Stop(data)
}

// GetRobotStatus 获取机器人状态
func (m *RobotManager) GetRobotStatus(robotID string) (interface{}, error) {
	m.mu.RLock()
	robot, exists := m.robots[robotID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("robot %s not found", robotID)
	}

	return robot.GetStatus()
}

// GetAllRobots 获取所有机器人状态
func (m *RobotManager) GetAllRobots() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})
	for id, robot := range m.robots {
		status, _ := robot.GetStatus()
		result[id] = status
	}
	return result
}

// DeleteRobot 删除机器人
func (m *RobotManager) DeleteRobot(robotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	robot, exists := m.robots[robotID]
	if !exists {
		return fmt.Errorf("robot %s not found", robotID)
	}

	// 先停止
	robot.Stop(nil)

	delete(m.robots, robotID)
	m.logger.Infof("Deleted robot: %s", robotID)
	return nil
}

// StopAll 停止所有机器人
func (m *RobotManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, robot := range m.robots {
		robot.Stop(nil)
		m.logger.Infof("Stopped robot: %s", id)
	}
}
