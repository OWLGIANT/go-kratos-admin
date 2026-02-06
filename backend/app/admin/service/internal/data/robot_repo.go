package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/predicate"
	"go-wind-admin/app/admin/service/internal/data/ent/robot"

	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

type RobotRepo interface {
	List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListRobotResponse, error)

	Get(ctx context.Context, req *tradingV1.GetRobotRequest) (*tradingV1.Robot, error)

	Create(ctx context.Context, req *tradingV1.CreateRobotRequest) (*tradingV1.Robot, error)

	Update(ctx context.Context, req *tradingV1.UpdateRobotRequest) error

	Delete(ctx context.Context, req *tradingV1.DeleteRobotRequest) error

	Count(ctx context.Context) (int, error)

	SyncRobot(ctx context.Context, tenantID uint32, rid string, status uint, balance float64) error

	GetRobot(ctx context.Context, tenantID uint32, rid string) (map[string]interface{}, error)
}

type robotRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[tradingV1.Robot, ent.Robot]

	repository *entCrud.Repository[
		ent.RobotQuery, ent.RobotSelect,
		ent.RobotCreate, ent.RobotCreateBulk,
		ent.RobotUpdate, ent.RobotUpdateOne,
		ent.RobotDelete,
		predicate.Robot,
		tradingV1.Robot, ent.Robot,
	]
}

func NewRobotRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
) RobotRepo {
	repo := &robotRepo{
		log:       ctx.NewLoggerHelper("robot/repo/admin-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[tradingV1.Robot, ent.Robot](),
	}

	repo.init()

	return repo
}

func (r *robotRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.RobotQuery, ent.RobotSelect,
		ent.RobotCreate, ent.RobotCreateBulk,
		ent.RobotUpdate, ent.RobotUpdateOne,
		ent.RobotDelete,
		predicate.Robot,
		tradingV1.Robot, ent.Robot,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Count 统计机器人数量
func (r *robotRepo) Count(ctx context.Context) (int, error) {
	builder := r.entClient.Client().Robot.Query()

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, err
	}

	return count, nil
}

// List 获取机器人列表
func (r *robotRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListRobotResponse, error) {
	builder := r.entClient.Client().Robot.Query()

	// 默认按ID降序排序
	builder.Order(ent.Desc(robot.FieldID))

	// 分页
	if req.Page != nil && req.PageSize != nil {
		offset := int((*req.Page - 1) * *req.PageSize)
		limit := int(*req.PageSize)
		builder.Offset(offset).Limit(limit)
	}

	// 查询
	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf("query list failed: %s", err.Error())
		return nil, err
	}

	// 转换
	items := make([]*tradingV1.Robot, 0, len(entities))
	for _, entity := range entities {
		item := r.entityToProto(entity)
		items = append(items, item)
	}

	// 统计总数
	total, err := r.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &tradingV1.ListRobotResponse{
		Total: int32(total),
		Items: items,
	}, nil
}

// entityToProto 将实体转换为 protobuf
func (r *robotRepo) entityToProto(entity *ent.Robot) *tradingV1.Robot {
	item := &tradingV1.Robot{
		RobotId:           entity.Rid,
		Nickname:          entity.Nickname,
		Exchange:          entity.Exchange,
		Version:           entity.Version,
		Status:            entity.Status,
		InitBalance:       entity.InitBalance,
		Balance:           entity.Balance,
		ServerId:          entity.ServerID,
		ExchangeAccountId: entity.ExchangeAccountID,
	}

	// 处理可选字段
	if entity.RegisteredAt != nil {
		item.RegisteredAt = timestamppb.New(*entity.RegisteredAt)
	}
	if entity.LastHeartbeat != nil {
		item.LastHeartbeat = timestamppb.New(*entity.LastHeartbeat)
	}

	return item
}

// Get 获取单个机器人
func (r *robotRepo) Get(ctx context.Context, req *tradingV1.GetRobotRequest) (*tradingV1.Robot, error) {
	entity, err := r.entClient.Client().Robot.Query().
		Where(robot.RidEQ(req.RobotId)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, err
		}
		r.log.Errorf("get robot failed: %s", err.Error())
		return nil, err
	}

	return r.entityToProto(entity), nil
}

// Create 创建机器人
func (r *robotRepo) Create(ctx context.Context, req *tradingV1.CreateRobotRequest) (*tradingV1.Robot, error) {
	builder := r.entClient.Client().Robot.Create()

	builder.
		SetRid(req.RobotId).
		SetNickname(req.Nickname).
		SetExchange(req.Exchange).
		SetVersion(req.Version).
		SetStatus(req.Status).
		SetInitBalance(req.InitBalance).
		SetBalance(req.Balance).
		SetServerID(req.ServerId).
		SetExchangeAccountID(req.ExchangeAccountId)

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create robot failed: %s", err.Error())
		return nil, err
	}

	return r.entityToProto(entity), nil
}

// Update 更新机器人
func (r *robotRepo) Update(ctx context.Context, req *tradingV1.UpdateRobotRequest) error {
	builder := r.entClient.Client().Robot.Update().
		Where(robot.RidEQ(req.RobotId))

	if req.Nickname != nil {
		builder.SetNickname(*req.Nickname)
	}
	if req.Exchange != nil {
		builder.SetExchange(*req.Exchange)
	}
	if req.Version != nil {
		builder.SetVersion(*req.Version)
	}
	if req.Status != nil {
		builder.SetStatus(*req.Status)
	}
	if req.InitBalance != nil {
		builder.SetInitBalance(*req.InitBalance)
	}
	if req.Balance != nil {
		builder.SetBalance(*req.Balance)
	}
	if req.ServerId != nil {
		builder.SetServerID(*req.ServerId)
	}
	if req.ExchangeAccountId != nil {
		builder.SetExchangeAccountID(*req.ExchangeAccountId)
	}

	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf("update robot failed: %s", err.Error())
		return err
	}

	return nil
}

// Delete 删除机器人
func (r *robotRepo) Delete(ctx context.Context, req *tradingV1.DeleteRobotRequest) error {
	_, err := r.entClient.Client().Robot.Delete().
		Where(robot.RidEQ(req.RobotId)).
		Exec(ctx)

	if err != nil {
		r.log.Errorf("delete robot failed: %s", err.Error())
		return err
	}

	return nil
}

// SyncRobot 同步机器人状态和余额
func (r *robotRepo) SyncRobot(ctx context.Context, tenantID uint32, rid string, status uint, balance float64) error {
	_, err := r.entClient.Client().Robot.Update().
		Where(robot.RidEQ(rid)).
		SetStatus(fmt.Sprintf("%d", status)).
		SetBalance(balance).
		Save(ctx)

	if err != nil {
		r.log.Errorf("sync robot failed: %s", err.Error())
		return err
	}

	return nil
}

// GetRobot 获取机器人信息（返回map格式）
func (r *robotRepo) GetRobot(ctx context.Context, tenantID uint32, rid string) (map[string]interface{}, error) {
	entity, err := r.entClient.Client().Robot.Query().
		Where(robot.RidEQ(rid)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, err
		}
		r.log.Errorf("get robot failed: %s", err.Error())
		return nil, err
	}

	result := map[string]interface{}{
		"robot_id":            entity.Rid,
		"nickname":            entity.Nickname,
		"exchange":            entity.Exchange,
		"version":             entity.Version,
		"status":              entity.Status,
		"init_balance":        entity.InitBalance,
		"balance":             entity.Balance,
		"registered_at":       entity.RegisteredAt,
		"last_heartbeat":      entity.LastHeartbeat,
		"server_id":           entity.ServerID,
		"exchange_account_id": entity.ExchangeAccountID,
	}

	return result, nil
}
