package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
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
	adminV1.ActorServiceHTTPServer

	log      *log.Helper
	registry ActorRegistry
}

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

// ListActor 获取 Actor 列表
func (s *ActorService) ListActor(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListActorResponse, error) {
	if s.registry == nil {
		s.log.Warn("Actor registry not set")
		return &tradingV1.ListActorResponse{
			Total: 0,
			Items: []*tradingV1.Actor{},
		}, nil
	}

	actors := s.registry.GetAll()
	s.log.Infof("ListActor: found %d actors", len(actors))

	items := make([]*tradingV1.Actor, 0, len(actors))
	for _, actor := range actors {
		items = append(items, s.convertActorInfo(actor))
	}

	return &tradingV1.ListActorResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

// GetActor 获取单个 Actor
func (s *ActorService) GetActor(ctx context.Context, req *tradingV1.GetActorRequest) (*tradingV1.Actor, error) {
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

// convertActorInfo 将内部 ActorInfo 转换为 proto Actor
func (s *ActorService) convertActorInfo(info *handler.ActorInfo) *tradingV1.Actor {
	actor := &tradingV1.Actor{
		ClientId:      info.ClientID,
		RobotId:       info.RobotID,
		Exchange:      info.Exchange,
		Version:       info.Version,
		TenantId:      info.TenantID,
		Status:        info.Status,
		Balance:       info.Balance,
		RegisteredAt:  timestamppb.New(info.RegisteredAt),
		LastHeartbeat: timestamppb.New(info.LastHeartbeat),
		Ip:            info.IP,
		InnerIp:       info.InnerIP,
		Port:          info.Port,
		MachineId:     info.MachineID,
		Nickname:      info.Nickname,
	}

	if info.ServerInfo != nil {
		actor.ServerInfo = &tradingV1.ServerStatusInfo{
			Cpu:               info.ServerInfo.CPU,
			IpPool:            info.ServerInfo.IPPool,
			Mem:               info.ServerInfo.Mem,
			MemPct:            info.ServerInfo.MemPct,
			DiskPct:           info.ServerInfo.DiskPct,
			TaskNum:           info.ServerInfo.TaskNum,
			StraVersion:       info.ServerInfo.StraVersion,
			StraVersionDetail: info.ServerInfo.StraVersionDetail,
			AwsAcct:           info.ServerInfo.AwsAcct,
			AwsZone:           info.ServerInfo.AwsZone,
		}
	}

	return actor
}
