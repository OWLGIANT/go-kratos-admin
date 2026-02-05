package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
	"go-wind-admin/app/admin/service/internal/websocket/handler"
)

// ActorRegistry 接口，用于获取 Actor 数据
type ActorRegistry interface {
	GetAll() []*handler.ActorInfo
	Get(robotID string) *handler.ActorInfo
}

type ActorService struct {
	log      *log.Helper
	registry ActorRegistry
}

// 确保 ActorService 实现了 RobotServiceHTTPServer 接口
var _ adminV1.RobotServiceHTTPServer = (*ActorService)(nil)

func NewActorService(
	ctx *bootstrap.Context,
) *ActorService {
	svc := &ActorService{
		log: ctx.NewLoggerHelper("actor/service/admin-service"),
	}

	return svc
}

// SetRegistry 设置 Actor Registry（由 WebSocket 服务器提供）
func (s *ActorService) SetRegistry(registry ActorRegistry) {
	s.registry = registry
}

// ListRobot 获取 Robot 列表
func (s *ActorService) ListRobot(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListRobotResponse, error) {
	if s.registry == nil {
		s.log.Warn("Actor registry not set")
		return &tradingV1.ListRobotResponse{
			Total: 0,
			Items: []*tradingV1.Robot{},
		}, nil
	}

	actors := s.registry.GetAll()
	s.log.Infof("ListRobot: found %d robots", len(actors))

	items := make([]*tradingV1.Robot, 0, len(actors))
	for _, actor := range actors {
		items = append(items, s.convertActorInfo(actor))
	}

	return &tradingV1.ListRobotResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

// GetRobot 获取单个 Robot
func (s *ActorService) GetRobot(ctx context.Context, req *tradingV1.GetRobotRequest) (*tradingV1.Robot, error) {
	if s.registry == nil {
		s.log.Warn("Actor registry not set")
		return nil, nil
	}

	actor := s.registry.Get(req.RobotId)
	if actor == nil {
		return nil, nil
	}

	return s.convertActorInfo(actor), nil
}

// convertActorInfo 将内部 ActorInfo 转换为 proto Robot
func (s *ActorService) convertActorInfo(info *handler.ActorInfo) *tradingV1.Robot {
	actor := &tradingV1.Robot{
		RobotId:       info.RobotID,
		Nickname:      info.Nickname,
		Exchange:      info.Exchange,
		Version:       info.Version,
		Status:        info.Status,
		Balance:       info.Balance,
		RegisteredAt:  timestamppb.New(info.RegisteredAt),
		LastHeartbeat: timestamppb.New(info.LastHeartbeat),
	}

	return actor
}

// CreateRobot 创建 Robot（Actor 是动态注册的，不支持手动创建）
func (s *ActorService) CreateRobot(ctx context.Context, req *tradingV1.CreateRobotRequest) (*emptypb.Empty, error) {
	s.log.Warn("CreateRobot not supported for actors")
	return &emptypb.Empty{}, nil
}

// UpdateRobot 更新 Robot（Actor 信息由 Actor 自己更新，不支持手动更新）
func (s *ActorService) UpdateRobot(ctx context.Context, req *tradingV1.UpdateRobotRequest) (*emptypb.Empty, error) {
	s.log.Warn("UpdateRobot not supported for actors")
	return &emptypb.Empty{}, nil
}

// DeleteRobot 删除 Robot（Actor 断开连接时自动删除，不支持手动删除）
func (s *ActorService) DeleteRobot(ctx context.Context, req *tradingV1.DeleteRobotRequest) (*emptypb.Empty, error) {
	s.log.Warn("DeleteRobot not supported for actors")
	return &emptypb.Empty{}, nil
}
