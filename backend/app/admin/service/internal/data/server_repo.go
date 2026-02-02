package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/predicate"
	"go-wind-admin/app/admin/service/internal/data/ent/server"

	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

type ServerRepo interface {
	List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListServerResponse, error)

	Get(ctx context.Context, req *tradingV1.GetServerRequest) (*tradingV1.Server, error)

	Create(ctx context.Context, req *tradingV1.CreateServerRequest) (*tradingV1.Server, error)

	BatchCreate(ctx context.Context, req *tradingV1.BatchCreateServerRequest) error

	Update(ctx context.Context, req *tradingV1.UpdateServerRequest) error

	Delete(ctx context.Context, req *tradingV1.DeleteServerRequest) error

	DeleteByIps(ctx context.Context, req *tradingV1.DeleteServerByIpsRequest) error

	Transfer(ctx context.Context, req *tradingV1.TransferServerRequest) error

	UpdateRemark(ctx context.Context, req *tradingV1.UpdateServerRemarkRequest) error

	UpdateStrategy(ctx context.Context, req *tradingV1.UpdateServerStrategyRequest) error

	GetCanRestartList(ctx context.Context, req *tradingV1.GetCanRestartServerListRequest) (*tradingV1.ListServerResponse, error)

	Count(ctx context.Context) (int, error)
}

type serverRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper           *mapper.CopierMapper[tradingV1.Server, ent.Server]
	serverTypeConverter *mapper.EnumTypeConverter[tradingV1.ServerType, int8]

	repository *entCrud.Repository[
		ent.ServerQuery, ent.ServerSelect,
		ent.ServerCreate, ent.ServerCreateBulk,
		ent.ServerUpdate, ent.ServerUpdateOne,
		ent.ServerDelete,
		predicate.Server,
		tradingV1.Server, ent.Server,
	]
}

func NewServerRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
) ServerRepo {
	repo := &serverRepo{
		log:                 ctx.NewLoggerHelper("server/repo/admin-service"),
		entClient:           entClient,
		mapper:              mapper.NewCopierMapper[tradingV1.Server, ent.Server](),
		serverTypeConverter: mapper.NewEnumTypeConverter[tradingV1.ServerType, int8](tradingV1.ServerType_name, tradingV1.ServerType_value),
	}

	repo.init()

	return repo
}

func (r *serverRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ServerQuery, ent.ServerSelect,
		ent.ServerCreate, ent.ServerCreateBulk,
		ent.ServerUpdate, ent.ServerUpdateOne,
		ent.ServerDelete,
		predicate.Server,
		tradingV1.Server, ent.Server,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Count 统计托管者数量
func (r *serverRepo) Count(ctx context.Context) (int, error) {
	builder := r.entClient.Client().Server.Query()

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, err
	}

	return count, nil
}

// List 获取托管者列表
func (r *serverRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListServerResponse, error) {
	builder := r.entClient.Client().Server.Query()

	// 应用过滤条件
	if req.Filter != nil {
		for _, condition := range req.Filter.Conditions {
			r.applyFilter(builder, condition)
		}
	}

	// 应用排序
	if req.OrderBy != nil && len(req.OrderBy) > 0 {
		for _, order := range req.OrderBy {
			r.applyOrder(builder, order)
		}
	} else {
		builder.Order(ent.Desc(server.FieldID))
	}

	// 分页
	if req.Pagination != nil {
		offset := int((req.Pagination.Page - 1) * req.Pagination.PageSize)
		limit := int(req.Pagination.PageSize)
		builder.Offset(offset).Limit(limit)
	}

	// 查询
	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf("query list failed: %s", err.Error())
		return nil, err
	}

	// 转换
	items := make([]*tradingV1.Server, 0, len(entities))
	for _, entity := range entities {
		item := &tradingV1.Server{}
		if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
			r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
			continue
		}
		r.unmarshalServer(item, entity)
		items = append(items, item)
	}

	// 统计总数
	total, err := r.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &tradingV1.ListServerResponse{
		Total: int32(total),
		Items: items,
	}, nil
}

// Get 获取单个托管者
func (r *serverRepo) Get(ctx context.Context, req *tradingV1.GetServerRequest) (*tradingV1.Server, error) {
	entity, err := r.entClient.Client().Server.Get(ctx, req.Id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, err
		}
		r.log.Errorf("get server failed: %s", err.Error())
		return nil, err
	}

	item := &tradingV1.Server{}
	if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
		r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
		return nil, err
	}

	r.unmarshalServer(item, entity)

	return item, nil
}

// Create 创建托管者
func (r *serverRepo) Create(ctx context.Context, req *tradingV1.CreateServerRequest) (*tradingV1.Server, error) {
	builder := r.entClient.Client().Server.Create()

	builder.
		SetNickname(req.Nickname).
		SetIP(req.Ip).
		SetInnerIP(req.InnerIp).
		SetPort(req.Port).
		SetMachineID(req.MachineId).
		SetRemark(req.Remark).
		SetVpcID(req.VpcId).
		SetInstanceID(req.InstanceId).
		SetType(int8(req.Type))

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create server failed: %s", err.Error())
		return nil, err
	}

	item := &tradingV1.Server{}
	if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
		r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
		return nil, err
	}

	return item, nil
}

// BatchCreate 批量创建托管者
func (r *serverRepo) BatchCreate(ctx context.Context, req *tradingV1.BatchCreateServerRequest) error {
	bulk := make([]*ent.ServerCreate, len(req.Servers))

	for i, serverReq := range req.Servers {
		bulk[i] = r.entClient.Client().Server.Create().
			SetNickname(serverReq.Nickname).
			SetIP(serverReq.Ip).
			SetInnerIP(serverReq.InnerIp).
			SetPort(serverReq.Port).
			SetMachineID(serverReq.MachineId).
			SetRemark(serverReq.Remark).
			SetVpcID(serverReq.VpcId).
			SetInstanceID(serverReq.InstanceId).
			SetType(int8(serverReq.Type))
	}

	_, err := r.entClient.Client().Server.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		r.log.Errorf("batch create servers failed: %s", err.Error())
		return err
	}

	return nil
}

// Update 更新托管者
func (r *serverRepo) Update(ctx context.Context, req *tradingV1.UpdateServerRequest) error {
	builder := r.entClient.Client().Server.UpdateOneID(req.Id)

	if req.Nickname != nil {
		builder.SetNickname(*req.Nickname)
	}
	if req.Ip != nil {
		builder.SetIP(*req.Ip)
	}
	if req.InnerIp != nil {
		builder.SetInnerIP(*req.InnerIp)
	}
	if req.Port != nil {
		builder.SetPort(*req.Port)
	}
	if req.MachineId != nil {
		builder.SetMachineID(*req.MachineId)
	}
	if req.Remark != nil {
		builder.SetRemark(*req.Remark)
	}
	if req.VpcId != nil {
		builder.SetVpcID(*req.VpcId)
	}
	if req.InstanceId != nil {
		builder.SetInstanceID(*req.InstanceId)
	}
	if req.Type != nil {
		builder.SetType(int8(*req.Type))
	}

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("update server failed: %s", err.Error())
		return err
	}

	return nil
}

// Delete 删除托管者
func (r *serverRepo) Delete(ctx context.Context, req *tradingV1.DeleteServerRequest) error {
	if err := r.entClient.Client().Server.DeleteOneID(req.Id).Exec(ctx); err != nil {
		r.log.Errorf("delete server failed: %s", err.Error())
		return err
	}

	return nil
}

// DeleteByIps 按IP删除托管者
func (r *serverRepo) DeleteByIps(ctx context.Context, req *tradingV1.DeleteServerByIpsRequest) error {
	_, err := r.entClient.Client().Server.Delete().
		Where(server.IPIn(req.Ips...)).
		Exec(ctx)

	if err != nil {
		r.log.Errorf("delete servers by ips failed: %s", err.Error())
		return err
	}

	return nil
}

// Transfer 转移托管者
func (r *serverRepo) Transfer(ctx context.Context, req *tradingV1.TransferServerRequest) error {
	_, err := r.entClient.Client().Server.Update().
		Where(server.IDIn(req.Ids...)).
		SetOperatorID(0). // 需要根据实际情况设置操作员ID
		Exec(ctx)

	if err != nil {
		r.log.Errorf("transfer servers failed: %s", err.Error())
		return err
	}

	return nil
}

// UpdateRemark 更新备注
func (r *serverRepo) UpdateRemark(ctx context.Context, req *tradingV1.UpdateServerRemarkRequest) error {
	return r.entClient.Client().Server.UpdateOneID(req.Id).
		SetRemark(req.Remark).
		Exec(ctx)
}

// UpdateStrategy 更新策略
func (r *serverRepo) UpdateStrategy(ctx context.Context, req *tradingV1.UpdateServerStrategyRequest) error {
	// 获取当前服务器信息
	entity, err := r.entClient.Client().Server.Get(ctx, req.Id)
	if err != nil {
		return err
	}

	// 更新服务器信息中的策略版本
	serverInfo := entity.ServerInfo
	if serverInfo == nil {
		serverInfo = make(map[string]interface{})
	}

	// 更新策略版本详情
	straVersionDetail, ok := serverInfo["stra_version_detail"].(map[string]interface{})
	if !ok {
		straVersionDetail = make(map[string]interface{})
	}
	straVersionDetail[req.StrategyName] = req.StrategyVersion
	serverInfo["stra_version_detail"] = straVersionDetail

	// 保存
	return r.entClient.Client().Server.UpdateOneID(req.Id).
		SetServerInfo(serverInfo).
		Exec(ctx)
}

// GetCanRestartList 获取可重启的托管者列表
func (r *serverRepo) GetCanRestartList(ctx context.Context, req *tradingV1.GetCanRestartServerListRequest) (*tradingV1.ListServerResponse, error) {
	builder := r.entClient.Client().Server.Query()

	// 根据操作员过滤
	if req.Operator != nil {
		// 需要根据实际情况实现
	}

	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf("get can restart server list failed: %s", err.Error())
		return nil, err
	}

	items := make([]*tradingV1.Server, 0, len(entities))
	for _, entity := range entities {
		item := &tradingV1.Server{}
		if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
			r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
			continue
		}
		r.unmarshalServer(item, entity)
		items = append(items, item)
	}

	return &tradingV1.ListServerResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

// applyFilter 应用过滤条件
func (r *serverRepo) applyFilter(builder *ent.ServerQuery, condition *paginationV1.FilterCondition) {
	field := condition.Field
	value := condition.Value

	switch field {
	case "type":
		if serverType, ok := tradingV1.ServerType_value[value]; ok {
			builder.Where(server.TypeEQ(int8(serverType)))
		}
	case "vpc_id":
		builder.Where(server.VpcIDEQ(value))
	case "ip":
		builder.Where(server.IPEQ(value))
	}
}

// applyOrder 应用排序
func (r *serverRepo) applyOrder(builder *ent.ServerQuery, order *paginationV1.OrderBy) {
	if order.Desc {
		switch order.Field {
		case "id":
			builder.Order(ent.Desc(server.FieldID))
		case "create_time":
			builder.Order(ent.Desc(server.FieldCreateTime))
		case "nickname":
			builder.Order(ent.Desc(server.FieldNickname))
		}
	} else {
		switch order.Field {
		case "id":
			builder.Order(ent.Asc(server.FieldID))
		case "create_time":
			builder.Order(ent.Asc(server.FieldCreateTime))
		case "nickname":
			builder.Order(ent.Asc(server.FieldNickname))
		}
	}
}

// unmarshalServer 解析服务器信息
func (r *serverRepo) unmarshalServer(item *tradingV1.Server, entity *ent.Server) {
	// 解析服务器状态信息
	if entity.ServerInfo != nil {
		serverInfo := &tradingV1.ServerStatusInfo{}

		if cpu, ok := entity.ServerInfo["cpu"].(string); ok {
			serverInfo.Cpu = cpu
		}
		if ipPool, ok := entity.ServerInfo["ip_pool"].(float64); ok {
			serverInfo.IpPool = ipPool
		}
		if mem, ok := entity.ServerInfo["mem"].(float64); ok {
			serverInfo.Mem = mem
		}
		if memPct, ok := entity.ServerInfo["mem_pct"].(string); ok {
			serverInfo.MemPct = memPct
		}
		if diskPct, ok := entity.ServerInfo["disk_pct"].(string); ok {
			serverInfo.DiskPct = diskPct
		}
		if taskNum, ok := entity.ServerInfo["task_num"].(float64); ok {
			serverInfo.TaskNum = int32(taskNum)
		}
		if straVersion, ok := entity.ServerInfo["stra_version"].(bool); ok {
			serverInfo.StraVersion = straVersion
		}
		if straVersionDetail, ok := entity.ServerInfo["stra_version_detail"].(map[string]interface{}); ok {
			serverInfo.StraVersionDetail = make(map[string]string)
			for k, v := range straVersionDetail {
				if strVal, ok := v.(string); ok {
					serverInfo.StraVersionDetail[k] = strVal
				}
			}
		}
		if awsAcct, ok := entity.ServerInfo["aws_acct"].(string); ok {
			serverInfo.AwsAcct = awsAcct
		}
		if awsZone, ok := entity.ServerInfo["aws_zone"].(string); ok {
			serverInfo.AwsZone = awsZone
		}

		item.ServerInfo = serverInfo
	}
}
