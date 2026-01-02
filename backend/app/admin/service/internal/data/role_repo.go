package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	pagination "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/predicate"
	"go-wind-admin/app/admin/service/internal/data/ent/role"

	userV1 "go-wind-admin/api/gen/go/user/service/v1"
)

type RoleRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper             *mapper.CopierMapper[userV1.Role, ent.Role]
	statusConverter    *mapper.EnumTypeConverter[userV1.Role_Status, role.Status]
	typeConverter      *mapper.EnumTypeConverter[userV1.Role_Type, role.Type]
	dataScopeConverter *mapper.EnumTypeConverter[userV1.Role_DataScope, role.DataScope]

	repository *entCrud.Repository[
		ent.RoleQuery, ent.RoleSelect,
		ent.RoleCreate, ent.RoleCreateBulk,
		ent.RoleUpdate, ent.RoleUpdateOne,
		ent.RoleDelete,
		predicate.Role,
		userV1.Role, ent.Role,
	]

	polePermissionRepo *RolePermissionRepo
	roleApiRepo        *RoleApiRepo
	roleMenuRepo       *RoleMenuRepo
}

func NewRoleRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
	polePermissionRepo *RolePermissionRepo,
	roleApiRepo *RoleApiRepo,
	roleMenuRepo *RoleMenuRepo,
) *RoleRepo {
	repo := &RoleRepo{
		log:       ctx.NewLoggerHelper("role/repo/admin-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[userV1.Role, ent.Role](),
		statusConverter: mapper.NewEnumTypeConverter[userV1.Role_Status, role.Status](
			userV1.Role_Status_name,
			userV1.Role_Status_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[userV1.Role_Type, role.Type](
			userV1.Role_Type_name,
			userV1.Role_Type_value,
		),
		dataScopeConverter: mapper.NewEnumTypeConverter[userV1.Role_DataScope, role.DataScope](
			userV1.Role_DataScope_name,
			userV1.Role_DataScope_value,
		),
		polePermissionRepo: polePermissionRepo,
		roleApiRepo:        roleApiRepo,
		roleMenuRepo:       roleMenuRepo,
	}

	repo.init()

	return repo
}

func (r *RoleRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.RoleQuery, ent.RoleSelect,
		ent.RoleCreate, ent.RoleCreateBulk,
		ent.RoleUpdate, ent.RoleUpdateOne,
		ent.RoleDelete,
		predicate.Role,
		userV1.Role, ent.Role,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
	r.mapper.AppendConverters(r.dataScopeConverter.NewConverterPair())
}

func (r *RoleRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Role.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, userV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func (r *RoleRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().Role.Query().
		Where(role.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, userV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *RoleRepo) List(ctx context.Context, req *pagination.PagingRequest) (*userV1.ListRoleResponse, error) {
	if req == nil {
		return nil, userV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Role.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &userV1.ListRoleResponse{Total: 0, Items: nil}, nil
	}

	if err = r.fillMenusAndApis(ctx, ret.Items); err != nil {
		return nil, err
	}

	return &userV1.ListRoleResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListRolesByRoleCodes 通过角色编码列表获取角色列表
func (r *RoleRepo) ListRolesByRoleCodes(ctx context.Context, codes []string) ([]*userV1.Role, error) {
	if len(codes) == 0 {
		return []*userV1.Role{}, nil
	}

	entities, err := r.entClient.Client().Role.Query().
		Where(role.CodeIn(codes...)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query roles by codes failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query roles by codes failed")
	}

	dtos := make([]*userV1.Role, 0, len(entities))
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		dtos = append(dtos, dto)
	}

	if err = r.fillMenusAndApis(ctx, dtos); err != nil {
		return nil, err
	}

	return dtos, nil
}

// ListRolesByRoleIds 通过角色ID列表获取角色列表
func (r *RoleRepo) ListRolesByRoleIds(ctx context.Context, ids []uint32) ([]*userV1.Role, error) {
	if len(ids) == 0 {
		return []*userV1.Role{}, nil
	}

	entities, err := r.entClient.Client().Role.Query().
		Where(role.IDIn(ids...)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query roles by ids failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query roles by ids failed")
	}

	dtos := make([]*userV1.Role, 0, len(entities))
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		dtos = append(dtos, dto)
	}

	if err = r.fillMenusAndApis(ctx, dtos); err != nil {
		return nil, err
	}

	return dtos, nil
}

// ListRoleCodesByRoleIds 通过角色ID列表获取角色编码列表
func (r *RoleRepo) ListRoleCodesByRoleIds(ctx context.Context, ids []uint32) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}

	entities, err := r.entClient.Client().Role.Query().
		Where(role.IDIn(ids...)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query role codes failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query role codes failed")
	}

	codes := make([]string, 0, len(entities))
	for _, entity := range entities {
		if entity.Code != nil {
			codes = append(codes, *entity.Code)
		}
	}

	return codes, nil
}

// Get 获取角色信息
func (r *RoleRepo) Get(ctx context.Context, req *userV1.GetRoleRequest) (*userV1.Role, error) {
	if req == nil {
		return nil, userV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Role.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *userV1.GetRoleRequest_Id:
		whereCond = append(whereCond, role.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	if err = r.fillMenuAndApi(ctx, dto); err != nil {
		return nil, err
	}

	return dto, err
}

// Create 创建角色
func (r *RoleRepo) Create(ctx context.Context, req *userV1.CreateRoleRequest) error {
	if req == nil || req.Data == nil {
		return userV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Role.Create().
		SetNillableName(req.Data.Name).
		SetNillableCode(req.Data.Code).
		SetNillableParentID(req.Data.ParentId).
		SetNillableTenantID(req.Data.TenantId).
		SetNillableSortOrder(req.Data.SortOrder).
		SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
		SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
		SetNillableDataScope(r.dataScopeConverter.ToEntity(req.Data.DataScope)).
		SetNillableRemark(req.Data.Remark).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetNillableCreatedAt(timeutil.TimestamppbToTime(req.Data.CreatedAt))

	if req.Data.TenantId == nil {
		builder.SetTenantID(req.Data.GetTenantId())
	}
	if req.Data.CreatedAt == nil {
		builder.SetCreatedAt(time.Now())
	}

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	var err error
	var ret *ent.Role
	if ret, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert one data failed: %s", err.Error())
		return userV1.ErrorInternalServerError("insert data failed")
	}

	if req.Data.Menus != nil {
		//builder.SetMenus(req.Data.Menus)
		if err = r.assignMenusToRole(ctx, ret.ID, req.Data.Menus, req.Data.GetCreatedBy()); err != nil {
			r.log.Errorf("assign menus to role failed: %s", err.Error())
			return userV1.ErrorInternalServerError("assign menus to role failed")
		}
	}
	if req.Data.Apis != nil {
		//builder.SetApis(req.Data.Apis)
		if err = r.assignApisToRole(ctx, ret.ID, req.Data.Apis, req.Data.GetCreatedBy()); err != nil {
			r.log.Errorf("assign apis to role failed: %s", err.Error())
			return userV1.ErrorInternalServerError("assign apis to role failed")
		}
	}

	return nil
}

// Update 更新角色信息
func (r *RoleRepo) Update(ctx context.Context, req *userV1.UpdateRoleRequest) error {
	if req == nil || req.Data == nil {
		return userV1.ErrorBadRequest("invalid parameter")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &userV1.CreateRoleRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Debug().Role.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *userV1.Role) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableParentID(req.Data.ParentId).
				SetNillableSortOrder(req.Data.SortOrder).
				SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
				SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
				SetNillableDataScope(r.dataScopeConverter.ToEntity(req.Data.DataScope)).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetNillableUpdatedAt(timeutil.TimestamppbToTime(req.Data.UpdatedAt))

			if req.Data.UpdatedAt == nil {
				builder.SetUpdatedAt(time.Now())
			}

		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(role.FieldID, req.GetId()))
		},
	)
	if err != nil {
		r.log.Errorf("update role failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update role failed")
	}

	if req.Data.Menus != nil {
		//builder.SetMenus(req.Data.Menus)
		if err = r.assignMenusToRole(ctx, req.GetId(), req.Data.Menus, req.Data.GetUpdatedBy()); err != nil {
			r.log.Errorf("assign menus to role failed: %s", err.Error())
			return userV1.ErrorInternalServerError("assign menus to role failed")
		}
	}
	if req.Data.Apis != nil {
		//builder.SetApis(req.Data.Apis)
		if err = r.assignApisToRole(ctx, req.GetId(), req.Data.Apis, req.Data.GetUpdatedBy()); err != nil {
			r.log.Errorf("assign apis to role failed: %s", err.Error())
			return userV1.ErrorInternalServerError("assign apis to role failed")
		}
	}

	return nil
}

// Delete 删除角色，同时删除其所有子角色
func (r *RoleRepo) Delete(ctx context.Context, req *userV1.DeleteRoleRequest) error {
	if req == nil {
		return userV1.ErrorBadRequest("invalid parameter")
	}

	ids, err := entCrud.QueryAllChildrenIds(ctx, r.entClient, "sys_roles", req.GetId())
	if err != nil {
		r.log.Errorf("query child roles failed: %s", err.Error())
		return userV1.ErrorInternalServerError("query child roles failed")
	}
	ids = append(ids, req.GetId())

	//r.log.Info("roles ids to delete: ", ids)

	if _, err = r.entClient.Client().Role.Delete().
		Where(role.IDIn(ids...)).
		Exec(ctx); err != nil {
		r.log.Errorf("delete roles failed: %s", err.Error())
		return userV1.ErrorInternalServerError("delete roles failed")
	}

	return nil
}

// assignApisToRole 分配API给角色
func (r *RoleRepo) assignApisToRole(ctx context.Context, roleId uint32, apiIds []uint32, operatorId uint32) error {
	return r.roleApiRepo.AssignApis(ctx, roleId, apiIds, operatorId)
}

// assignMenusToRole 分配菜单给角色
func (r *RoleRepo) assignMenusToRole(ctx context.Context, roleId uint32, menuIds []uint32, operatorId uint32) error {
	return r.roleMenuRepo.AssignMenus(ctx, roleId, menuIds, operatorId)
}

func (r *RoleRepo) fillMenuAndApi(ctx context.Context, role *userV1.Role) error {
	// 获取角色分配的菜单ID列表
	menuIds, err := r.roleMenuRepo.ListMenuIdsByRoleId(ctx, role.GetId())
	if err != nil {
		r.log.Errorf("list menu ids by role id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("list menu ids by role id failed")
	}
	role.Menus = menuIds

	// 获取角色分配的API ID列表
	apiIds, err := r.roleApiRepo.ListApiIdsByRoleId(ctx, role.GetId())
	if err != nil {
		r.log.Errorf("list api ids by role id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("list api ids by role id failed")
	}
	role.Apis = apiIds

	return nil
}

// fillMenusAndApis 填充角色的菜单和API列表
func (r *RoleRepo) fillMenusAndApis(ctx context.Context, roles []*userV1.Role) error {
	for _, item := range roles {
		if err := r.fillMenuAndApi(ctx, item); err != nil {
			return err
		}
	}

	return nil
}
