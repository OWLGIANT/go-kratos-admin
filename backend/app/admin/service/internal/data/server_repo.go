package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	UpsertByIP(ctx context.Context, req *tradingV1.UpsertServerByIPRequest) (*tradingV1.Server, error)

	Count(ctx context.Context) (int, error)
}

type serverRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[tradingV1.Server, ent.Server]

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
		log:       ctx.NewLoggerHelper("server/repo/admin-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[tradingV1.Server, ent.Server](),
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

	// 默认按ID降序排序
	builder.Order(ent.Desc(server.FieldID))

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
	items := make([]*tradingV1.Server, 0, len(entities))
	for _, entity := range entities {
		item := r.entityToProto(entity)
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

// entityToProto 将实体转换为 protobuf
func (r *serverRepo) entityToProto(entity *ent.Server) *tradingV1.Server {
	item := &tradingV1.Server{
		Id:       entity.ID,
		Nickname: entity.Nickname,
		Ip:       entity.IP,
		InnerIp:  entity.InnerIP,
		Port:     entity.Port,
		Type:     tradingV1.ServerType(entity.Type),
	}

	// 处理可选字段
	if entity.MachineID != nil {
		item.MachineId = *entity.MachineID
	}
	if entity.Remark != nil {
		item.Remark = *entity.Remark
	}
	if entity.VpcID != nil {
		item.VpcId = *entity.VpcID
	}
	if entity.InstanceID != nil {
		item.InstanceId = *entity.InstanceID
	}

	// 处理时间字段
	if entity.CreatedAt != nil {
		item.CreateTime = timestamppb.New(*entity.CreatedAt)
	}
	if entity.UpdatedAt != nil {
		item.UpdateTime = timestamppb.New(*entity.UpdatedAt)
	}

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

		item.ServerInfo = serverInfo
	}

	return item
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

	return r.entityToProto(entity), nil
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

	return r.entityToProto(entity), nil
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
	// TODO: 需要根据实际情况实现，当前 ent schema 中没有 OperatorID 字段
	r.log.Warnf("transfer servers not fully implemented")
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
		item := r.entityToProto(entity)
		items = append(items, item)
	}

	return &tradingV1.ListServerResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

// UpsertByIP 根据IP插入或更新服务器信息
func (r *serverRepo) UpsertByIP(ctx context.Context, req *tradingV1.UpsertServerByIPRequest) (*tradingV1.Server, error) {
	// 先查询是否存在该IP的服务器
	existingServer, err := r.entClient.Client().Server.Query().
		Where(server.IPEQ(req.Ip)).
		Only(ctx)

	if err != nil && !ent.IsNotFound(err) {
		r.log.Errorf("query server by ip failed: %s", err.Error())
		return nil, err
	}

	// 准备服务器信息JSON
	var serverInfo map[string]interface{}
	if req.ServerInfo != nil {
		serverInfo = make(map[string]interface{})
		serverInfo["cpu"] = req.ServerInfo.Cpu
		serverInfo["ip_pool"] = req.ServerInfo.IpPool
		serverInfo["mem"] = req.ServerInfo.Mem
		serverInfo["mem_pct"] = req.ServerInfo.MemPct
		serverInfo["disk_pct"] = req.ServerInfo.DiskPct
		serverInfo["task_num"] = req.ServerInfo.TaskNum
	}

	if existingServer != nil {
		// 更新现有服务器
		builder := r.entClient.Client().Server.UpdateOneID(existingServer.ID)

		if req.Nickname != "" {
			builder.SetNickname(req.Nickname)
		}
		if req.InnerIp != "" {
			builder.SetInnerIP(req.InnerIp)
		}
		if req.Port != "" {
			builder.SetPort(req.Port)
		}
		if req.MachineId != "" {
			builder.SetMachineID(req.MachineId)
		}
		if serverInfo != nil {
			builder.SetServerInfo(serverInfo)
		}

		if err := builder.Exec(ctx); err != nil {
			r.log.Errorf("update server by ip failed: %s", err.Error())
			return nil, err
		}

		// 重新查询更新后的数据
		updatedServer, err := r.entClient.Client().Server.Get(ctx, existingServer.ID)
		if err != nil {
			return nil, err
		}
		return r.entityToProto(updatedServer), nil
	}

	// 创建新服务器
	builder := r.entClient.Client().Server.Create().
		SetIP(req.Ip).
		SetInnerIP(req.InnerIp).
		SetPort(req.Port).
		SetNickname(req.Nickname).
		SetMachineID(req.MachineId)

	if serverInfo != nil {
		builder.SetServerInfo(serverInfo)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create server failed: %s", err.Error())
		return nil, err
	}

	return r.entityToProto(entity), nil
}
