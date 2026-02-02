package data

import (
	"context"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/exchangeaccount"
	"go-wind-admin/app/admin/service/internal/data/ent/predicate"

	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

type ExchangeAccountRepo interface {
	List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListExchangeAccountResponse, error)

	Get(ctx context.Context, req *tradingV1.GetExchangeAccountRequest) (*tradingV1.ExchangeAccount, error)

	Create(ctx context.Context, req *tradingV1.CreateExchangeAccountRequest) (*tradingV1.ExchangeAccount, error)

	Update(ctx context.Context, req *tradingV1.UpdateExchangeAccountRequest) error

	Delete(ctx context.Context, req *tradingV1.DeleteExchangeAccountRequest) error

	BatchDelete(ctx context.Context, req *tradingV1.BatchDeleteExchangeAccountRequest) error

	Transfer(ctx context.Context, req *tradingV1.TransferExchangeAccountRequest) error

	Search(ctx context.Context, req *tradingV1.SearchExchangeAccountRequest) (*tradingV1.ListExchangeAccountResponse, error)

	UpdateRemark(ctx context.Context, req *tradingV1.UpdateAccountRemarkRequest) error

	UpdateBrokerId(ctx context.Context, req *tradingV1.UpdateAccountBrokerIdRequest) error

	CreateCombined(ctx context.Context, req *tradingV1.CreateCombinedAccountRequest) (*tradingV1.ExchangeAccount, error)

	UpdateCombined(ctx context.Context, req *tradingV1.UpdateCombinedAccountRequest) error

	Count(ctx context.Context) (int, error)
}

type exchangeAccountRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper              *mapper.CopierMapper[tradingV1.ExchangeAccount, ent.ExchangeAccount]
	accountTypeConverter *mapper.EnumTypeConverter[tradingV1.AccountType, int8]

	repository *entCrud.Repository[
		ent.ExchangeAccountQuery, ent.ExchangeAccountSelect,
		ent.ExchangeAccountCreate, ent.ExchangeAccountCreateBulk,
		ent.ExchangeAccountUpdate, ent.ExchangeAccountUpdateOne,
		ent.ExchangeAccountDelete,
		predicate.ExchangeAccount,
		tradingV1.ExchangeAccount, ent.ExchangeAccount,
	]
}

func NewExchangeAccountRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
) ExchangeAccountRepo {
	repo := &exchangeAccountRepo{
		log:                  ctx.NewLoggerHelper("exchange-account/repo/admin-service"),
		entClient:            entClient,
		mapper:               mapper.NewCopierMapper[tradingV1.ExchangeAccount, ent.ExchangeAccount](),
		accountTypeConverter: mapper.NewEnumTypeConverter[tradingV1.AccountType, int8](tradingV1.AccountType_name, tradingV1.AccountType_value),
	}

	repo.init()

	return repo
}

func (r *exchangeAccountRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ExchangeAccountQuery, ent.ExchangeAccountSelect,
		ent.ExchangeAccountCreate, ent.ExchangeAccountCreateBulk,
		ent.ExchangeAccountUpdate, ent.ExchangeAccountUpdateOne,
		ent.ExchangeAccountDelete,
		predicate.ExchangeAccount,
		tradingV1.ExchangeAccount, ent.ExchangeAccount,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Count 统计账号数量
func (r *exchangeAccountRepo) Count(ctx context.Context) (int, error) {
	builder := r.entClient.Client().ExchangeAccount.Query()

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, err
	}

	return count, nil
}

// List 获取账号列表
func (r *exchangeAccountRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListExchangeAccountResponse, error) {
	builder := r.entClient.Client().ExchangeAccount.Query()

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
		builder.Order(ent.Desc(exchangeaccount.FieldID))
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
	items := make([]*tradingV1.ExchangeAccount, 0, len(entities))
	for _, entity := range entities {
		item := &tradingV1.ExchangeAccount{}
		if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
			r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
			continue
		}
		// 解密敏感信息（如果需要）
		r.unmarshalAccount(item, entity)
		items = append(items, item)
	}

	// 统计总数
	total, err := r.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &tradingV1.ListExchangeAccountResponse{
		Total: int32(total),
		Items: items,
	}, nil
}

// Get 获取单个账号
func (r *exchangeAccountRepo) Get(ctx context.Context, req *tradingV1.GetExchangeAccountRequest) (*tradingV1.ExchangeAccount, error) {
	entity, err := r.entClient.Client().ExchangeAccount.Get(ctx, req.Id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, err
		}
		r.log.Errorf("get exchange account failed: %s", err.Error())
		return nil, err
	}

	item := &tradingV1.ExchangeAccount{}
	if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
		r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
		return nil, err
	}

	r.unmarshalAccount(item, entity)

	return item, nil
}

// Create 创建账号
func (r *exchangeAccountRepo) Create(ctx context.Context, req *tradingV1.CreateExchangeAccountRequest) (*tradingV1.ExchangeAccount, error) {
	builder := r.entClient.Client().ExchangeAccount.Create()

	builder.
		SetNickname(req.Nickname).
		SetExchangeName(req.ExchangeName).
		SetOriginAccount(req.OriginAccount).
		SetApiKey(req.ApiKey).
		SetSecretKey(req.SecretKey).
		SetPassKey(req.PassKey).
		SetBrokerID(req.BrokerId).
		SetRemark(req.Remark).
		SetServerIps(req.ServerIps).
		SetSpecialReqLimit(req.SpecialReqLimit).
		SetAccountType(int8(req.AccountType))

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create exchange account failed: %s", err.Error())
		return nil, err
	}

	item := &tradingV1.ExchangeAccount{}
	if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
		r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
		return nil, err
	}

	return item, nil
}

// Update 更新账号
func (r *exchangeAccountRepo) Update(ctx context.Context, req *tradingV1.UpdateExchangeAccountRequest) error {
	builder := r.entClient.Client().ExchangeAccount.UpdateOneID(req.Id)

	if req.Nickname != nil {
		builder.SetNickname(*req.Nickname)
	}
	if req.Remark != nil {
		builder.SetRemark(*req.Remark)
	}
	if req.ServerIps != nil {
		builder.SetServerIps(*req.ServerIps)
	}
	if req.SpecialReqLimit != nil {
		builder.SetSpecialReqLimit(*req.SpecialReqLimit)
	}
	if req.ApiKey != nil {
		builder.SetApiKey(*req.ApiKey)
	}
	if req.SecretKey != nil {
		builder.SetSecretKey(*req.SecretKey)
	}
	if req.PassKey != nil {
		builder.SetPassKey(*req.PassKey)
	}
	if req.BrokerId != nil {
		builder.SetBrokerID(*req.BrokerId)
	}

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("update exchange account failed: %s", err.Error())
		return err
	}

	return nil
}

// Delete 删除账号
func (r *exchangeAccountRepo) Delete(ctx context.Context, req *tradingV1.DeleteExchangeAccountRequest) error {
	if err := r.entClient.Client().ExchangeAccount.DeleteOneID(req.Id).Exec(ctx); err != nil {
		r.log.Errorf("delete exchange account failed: %s", err.Error())
		return err
	}

	return nil
}

// BatchDelete 批量删除账号
func (r *exchangeAccountRepo) BatchDelete(ctx context.Context, req *tradingV1.BatchDeleteExchangeAccountRequest) error {
	_, err := r.entClient.Client().ExchangeAccount.Delete().
		Where(exchangeaccount.APIKeyIn(req.ApiKeys...)).
		Exec(ctx)

	if err != nil {
		r.log.Errorf("batch delete exchange accounts failed: %s", err.Error())
		return err
	}

	return nil
}

// Transfer 转移账号
func (r *exchangeAccountRepo) Transfer(ctx context.Context, req *tradingV1.TransferExchangeAccountRequest) error {
	_, err := r.entClient.Client().ExchangeAccount.Update().
		Where(exchangeaccount.IDIn(req.Ids...)).
		SetOperatorID(0). // 需要根据实际情况设置操作员ID
		Exec(ctx)

	if err != nil {
		r.log.Errorf("transfer exchange accounts failed: %s", err.Error())
		return err
	}

	return nil
}

// Search 搜索账号
func (r *exchangeAccountRepo) Search(ctx context.Context, req *tradingV1.SearchExchangeAccountRequest) (*tradingV1.ListExchangeAccountResponse, error) {
	builder := r.entClient.Client().ExchangeAccount.Query()

	// 关键词搜索
	if req.Keyword != "" {
		builder.Where(
			exchangeaccount.Or(
				exchangeaccount.NicknameContains(req.Keyword),
				exchangeaccount.ExchangeNameContains(req.Keyword),
				exchangeaccount.OriginAccountContains(req.Keyword),
			),
		)
	}

	// 操作员过滤
	if req.Operator != nil {
		// 需要根据实际情况实现
	}

	// 账号类型过滤
	if req.AccountType != nil {
		builder.Where(exchangeaccount.AccountTypeEQ(int8(*req.AccountType)))
	}

	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf("search exchange accounts failed: %s", err.Error())
		return nil, err
	}

	items := make([]*tradingV1.ExchangeAccount, 0, len(entities))
	for _, entity := range entities {
		item := &tradingV1.ExchangeAccount{}
		if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
			r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
			continue
		}
		r.unmarshalAccount(item, entity)
		items = append(items, item)
	}

	return &tradingV1.ListExchangeAccountResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

// UpdateRemark 更新备注
func (r *exchangeAccountRepo) UpdateRemark(ctx context.Context, req *tradingV1.UpdateAccountRemarkRequest) error {
	return r.entClient.Client().ExchangeAccount.UpdateOneID(req.Id).
		SetRemark(req.Remark).
		Exec(ctx)
}

// UpdateBrokerId 更新经纪商ID
func (r *exchangeAccountRepo) UpdateBrokerId(ctx context.Context, req *tradingV1.UpdateAccountBrokerIdRequest) error {
	builder := r.entClient.Client().ExchangeAccount.Update()

	if len(req.ApiKeys) > 0 {
		builder.Where(exchangeaccount.APIKeyIn(req.ApiKeys...))
	}
	if req.ExchangeName != "" {
		builder.Where(exchangeaccount.ExchangeNameEQ(req.ExchangeName))
	}
	if req.OriginAccount != "" {
		builder.Where(exchangeaccount.OriginAccountEQ(req.OriginAccount))
	}

	_, err := builder.SetBrokerID(req.BrokerId).Save(ctx)
	return err
}

// CreateCombined 创建组合账号
func (r *exchangeAccountRepo) CreateCombined(ctx context.Context, req *tradingV1.CreateCombinedAccountRequest) (*tradingV1.ExchangeAccount, error) {
	// 将子账号ID列表转换为字符串
	combinedID := strings.Join(convertUint32ToStringSlice(req.AccountIds), "|")

	builder := r.entClient.Client().ExchangeAccount.Create()
	builder.
		SetNickname(req.Nickname).
		SetExchangeName("COMBINED").
		SetOriginAccount("COMBINED").
		SetApiKey("COMBINED_" + combinedID).
		SetSecretKey("").
		SetIsMulti(true).
		SetCombinedID(combinedID).
		SetRemark(req.Remark)

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create combined account failed: %s", err.Error())
		return nil, err
	}

	// 更新子账号
	_, err = r.entClient.Client().ExchangeAccount.Update().
		Where(exchangeaccount.IDIn(req.AccountIds...)).
		SetIsCombined(true).
		Save(ctx)

	if err != nil {
		r.log.Errorf("update sub accounts failed: %s", err.Error())
		return nil, err
	}

	item := &tradingV1.ExchangeAccount{}
	if err := r.mapper.EntityToProtobuf(entity, item); err != nil {
		r.log.Errorf("convert entity to protobuf failed: %s", err.Error())
		return nil, err
	}

	return item, nil
}

// UpdateCombined 更新组合账号
func (r *exchangeAccountRepo) UpdateCombined(ctx context.Context, req *tradingV1.UpdateCombinedAccountRequest) error {
	builder := r.entClient.Client().ExchangeAccount.UpdateOneID(req.Id)

	if req.Nickname != nil {
		builder.SetNickname(*req.Nickname)
	}
	if req.Remark != nil {
		builder.SetRemark(*req.Remark)
	}
	if len(req.AccountIds) > 0 {
		combinedID := strings.Join(convertUint32ToStringSlice(req.AccountIds), "|")
		builder.SetCombinedID(combinedID)
	}

	return builder.Exec(ctx)
}

// applyFilter 应用过滤条件
func (r *exchangeAccountRepo) applyFilter(builder *ent.ExchangeAccountQuery, condition *paginationV1.FilterCondition) {
	field := condition.Field
	value := condition.Value

	switch field {
	case "exchange_name":
		builder.Where(exchangeaccount.ExchangeNameEQ(value))
	case "account_type":
		if accountType, ok := tradingV1.AccountType_value[value]; ok {
			builder.Where(exchangeaccount.AccountTypeEQ(int8(accountType)))
		}
	case "is_multi":
		if value == "true" {
			builder.Where(exchangeaccount.IsMultiEQ(true))
		} else {
			builder.Where(exchangeaccount.IsMultiEQ(false))
		}
	}
}

// applyOrder 应用排序
func (r *exchangeAccountRepo) applyOrder(builder *ent.ExchangeAccountQuery, order *paginationV1.OrderBy) {
	if order.Desc {
		switch order.Field {
		case "id":
			builder.Order(ent.Desc(exchangeaccount.FieldID))
		case "create_time":
			builder.Order(ent.Desc(exchangeaccount.FieldCreateTime))
		case "nickname":
			builder.Order(ent.Desc(exchangeaccount.FieldNickname))
		}
	} else {
		switch order.Field {
		case "id":
			builder.Order(ent.Asc(exchangeaccount.FieldID))
		case "create_time":
			builder.Order(ent.Asc(exchangeaccount.FieldCreateTime))
		case "nickname":
			builder.Order(ent.Asc(exchangeaccount.FieldNickname))
		}
	}
}

// unmarshalAccount 解析账号信息
func (r *exchangeAccountRepo) unmarshalAccount(item *tradingV1.ExchangeAccount, entity *ent.ExchangeAccount) {
	// 解析组合账号ID
	if entity.CombinedID != "" {
		accountIDs := strings.Split(entity.CombinedID, "|")
		item.AccountIds = accountIDs
	}

	// 清除敏感信息（不返回给前端）
	item.SecretKey = ""
	item.PassKey = ""
}

// convertUint32ToStringSlice 转换uint32切片为字符串切片
func convertUint32ToStringSlice(ids []uint32) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(rune(id))
	}
	return result
}
