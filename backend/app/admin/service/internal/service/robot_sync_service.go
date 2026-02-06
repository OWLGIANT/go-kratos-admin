package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data"
)

// RobotSyncService handles robot synchronization business logic
type RobotSyncService struct {
	robotRepo data.RobotRepo
	log       *log.Helper
}

// NewRobotSyncService creates a new robot sync service
func NewRobotSyncService(ctx *bootstrap.Context, robotRepo data.RobotRepo) *RobotSyncService {
	return &RobotSyncService{
		robotRepo: robotRepo,
		log:       ctx.NewLoggerHelper("robot-sync/service/admin-service"),
	}
}

// SyncRobot synchronizes robot data
func (s *RobotSyncService) SyncRobot(ctx context.Context, tenantID uint32, rid string, status uint, balance float64) error {
	s.log.Infof("Syncing robot: rid=%s, status=%d, balance=%.2f, tenant=%d", rid, status, balance, tenantID)

	// Validate input
	if rid == "" {
		s.log.Error("Robot ID is required")
		return nil // TODO: Return proper error once error types are defined
	}

	// Update robot in database
	if err := s.robotRepo.SyncRobot(ctx, tenantID, rid, status, balance); err != nil {
		s.log.Errorf("Failed to sync robot %s: %v", rid, err)
		return err
	}

	s.log.Infof("Robot synced successfully: rid=%s", rid)
	return nil
}

// GetRobot retrieves robot information
func (s *RobotSyncService) GetRobot(ctx context.Context, tenantID uint32, rid string) (map[string]interface{}, error) {
	s.log.Infof("Getting robot: rid=%s, tenant=%d", rid, tenantID)

	robot, err := s.robotRepo.GetRobot(ctx, tenantID, rid)
	if err != nil {
		s.log.Errorf("Failed to get robot %s: %v", rid, err)
		return nil, err
	}

	return robot, nil
}
