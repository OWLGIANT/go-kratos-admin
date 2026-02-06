package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-admin/app/admin/service/internal/data"
	"go-wind-admin/app/admin/service/internal/websocket/handler"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

// ActorRegistry 接口，用于获取 Actor 数据
type ActorRegistry interface {
	GetAll() []*handler.ActorServerInfo
	Get(robotID string) *handler.ActorServerInfo
}

type ServerService struct {
	adminV1.ServerServiceHTTPServer

	log *log.Helper

	serverRepo data.ServerRepo
	registry   ActorRegistry
}

func NewServerService(
	ctx *bootstrap.Context,
	serverRepo data.ServerRepo,
) *ServerService {
	svc := &ServerService{
		log:        ctx.NewLoggerHelper("server/service/admin-service"),
		serverRepo: serverRepo,
	}

	return svc
}

// SetRegistry 设置 Actor Registry（由 WebSocket 服务器提供）
func (s *ServerService) SetRegistry(registry ActorRegistry) {
	s.registry = registry
}

// ListServer 获取托管者列表
func (s *ServerService) ListServer(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListServerResponse, error) {
	if s.registry == nil {
		s.log.Warn("Actor registry not set")
		return &tradingV1.ListServerResponse{
			Total: 0,
			Items: []*tradingV1.Server{},
		}, nil
	}
	return s.serverRepo.List(ctx, req)
}

// GetServer 获取托管者
func (s *ServerService) GetServer(ctx context.Context, req *tradingV1.GetServerRequest) (*tradingV1.Server, error) {
	return s.serverRepo.Get(ctx, req)
}

// CreateServer 创建托管者
func (s *ServerService) CreateServer(ctx context.Context, req *tradingV1.CreateServerRequest) (*emptypb.Empty, error) {
	_, err := s.serverRepo.Create(ctx, req)
	if err != nil {
		s.log.Errorf("create server failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// BatchCreateServer 批量创建托管者
func (s *ServerService) BatchCreateServer(ctx context.Context, req *tradingV1.BatchCreateServerRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.BatchCreate(ctx, req)
	if err != nil {
		s.log.Errorf("batch create servers failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateServer 更新托管者
func (s *ServerService) UpdateServer(ctx context.Context, req *tradingV1.UpdateServerRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.Update(ctx, req)
	if err != nil {
		s.log.Errorf("update server failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteServer 删除托管者
func (s *ServerService) DeleteServer(ctx context.Context, req *tradingV1.DeleteServerRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.Delete(ctx, req)
	if err != nil {
		s.log.Errorf("delete server failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteServerByIps 按IP删除托管者
func (s *ServerService) DeleteServerByIps(ctx context.Context, req *tradingV1.DeleteServerByIpsRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.DeleteByIps(ctx, req)
	if err != nil {
		s.log.Errorf("delete servers by ips failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// RebootServer 重启托管者
func (s *ServerService) RebootServer(ctx context.Context, req *tradingV1.RebootServerRequest) (*emptypb.Empty, error) {
	// TODO: 实现重启逻辑
	// 这需要调用远程服务器的重启接口
	s.log.Infof("reboot server: %d", req.Id)

	return &emptypb.Empty{}, nil
}

// GetServerLog 获取托管者日志
func (s *ServerService) GetServerLog(ctx context.Context, req *tradingV1.GetServerLogRequest) (*tradingV1.GetServerLogResponse, error) {
	// TODO: 实现日志获取逻辑
	// 这需要从远程服务器获取日志
	return &tradingV1.GetServerLogResponse{
		LogContent: "",
	}, nil
}

// StopServerRobot 停止托管者上的机器人
func (s *ServerService) StopServerRobot(ctx context.Context, req *tradingV1.StopServerRobotRequest) (*emptypb.Empty, error) {
	// TODO: 实现停止机器人逻辑
	// 这需要调用远程服务器的停止接口
	s.log.Infof("stop robot %s on server %d", req.RobotId, req.Id)

	return &emptypb.Empty{}, nil
}

// TransferServer 转移托管者
func (s *ServerService) TransferServer(ctx context.Context, req *tradingV1.TransferServerRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.Transfer(ctx, req)
	if err != nil {
		s.log.Errorf("transfer servers failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteServerLog 删除托管者日志
func (s *ServerService) DeleteServerLog(ctx context.Context, req *tradingV1.DeleteServerLogRequest) (*emptypb.Empty, error) {
	// TODO: 实现删除日志逻辑
	// 这需要调用远程服务器的删除日志接口
	s.log.Infof("delete log for server %d", req.Id)

	return &emptypb.Empty{}, nil
}

// UpdateServerStrategy 更新托管者策略
func (s *ServerService) UpdateServerStrategy(ctx context.Context, req *tradingV1.UpdateServerStrategyRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.UpdateStrategy(ctx, req)
	if err != nil {
		s.log.Errorf("update server strategy failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateServerRemark 更新托管者备注
func (s *ServerService) UpdateServerRemark(ctx context.Context, req *tradingV1.UpdateServerRemarkRequest) (*emptypb.Empty, error) {
	err := s.serverRepo.UpdateRemark(ctx, req)
	if err != nil {
		s.log.Errorf("update server remark failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// GetCanRestartServerList 获取可重启的托管者列表
func (s *ServerService) GetCanRestartServerList(ctx context.Context, req *tradingV1.GetCanRestartServerListRequest) (*tradingV1.ListServerResponse, error) {
	return s.serverRepo.GetCanRestartList(ctx, req)
}
