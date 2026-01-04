package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pagination "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/permission"
	"go-wind-admin/app/admin/service/internal/data/ent/predicate"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
)

type PermissionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper          *mapper.CopierMapper[adminV1.Permission, ent.Permission]
	typeConverter   *mapper.EnumTypeConverter[adminV1.Permission_Type, permission.Type]
	statusConverter *mapper.EnumTypeConverter[adminV1.Permission_Status, permission.Status]

	repository *entCrud.Repository[
		ent.PermissionQuery, ent.PermissionSelect,
		ent.PermissionCreate, ent.PermissionCreateBulk,
		ent.PermissionUpdate, ent.PermissionUpdateOne,
		ent.PermissionDelete,
		predicate.Permission,
		adminV1.Permission, ent.Permission,
	]

	permissionApiResourceRepo *PermissionApiResourceRepo
	permissionMenuRepo        *PermissionMenuRepo
}

func NewPermissionRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
	permissionApiResourceRepo *PermissionApiResourceRepo,
	permissionMenuRepo *PermissionMenuRepo,
) *PermissionRepo {
	repo := &PermissionRepo{
		log:                       ctx.NewLoggerHelper("permission/repo/admin-service"),
		entClient:                 entClient,
		mapper:                    mapper.NewCopierMapper[adminV1.Permission, ent.Permission](),
		typeConverter:             mapper.NewEnumTypeConverter[adminV1.Permission_Type, permission.Type](adminV1.Permission_Type_name, adminV1.Permission_Type_value),
		statusConverter:           mapper.NewEnumTypeConverter[adminV1.Permission_Status, permission.Status](adminV1.Permission_Status_name, adminV1.Permission_Status_value),
		permissionApiResourceRepo: permissionApiResourceRepo,
		permissionMenuRepo:        permissionMenuRepo,
	}

	repo.init()

	return repo
}

func (r *PermissionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.PermissionQuery, ent.PermissionSelect,
		ent.PermissionCreate, ent.PermissionCreateBulk,
		ent.PermissionUpdate, ent.PermissionUpdateOne,
		ent.PermissionDelete,
		predicate.Permission,
		adminV1.Permission, ent.Permission,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
}

func (r *PermissionRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Permission.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, adminV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func isApiPermission(typ adminV1.Permission_Type) bool {
	if typ == adminV1.Permission_API {
		return true
	}
	return false
}

func isMenuPermission(typ adminV1.Permission_Type) bool {
	if typ == adminV1.Permission_CATALOG ||
		typ == adminV1.Permission_MENU ||
		typ == adminV1.Permission_BUTTON ||
		typ == adminV1.Permission_PAGE {
		return true
	}
	return false
}

func (r *PermissionRepo) List(ctx context.Context, req *pagination.PagingRequest) (*adminV1.ListPermissionResponse, error) {
	if req == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Permission.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &adminV1.ListPermissionResponse{Total: 0, Items: nil}, nil
	}

	hasMenuID := hasPath("menu_id", req.GetFieldMask())
	hasApiResourceID := hasPath("api_resource_id", req.GetFieldMask())

	for _, dto := range ret.Items {
		if hasMenuID && isMenuPermission(dto.GetType()) {
			menuID, err := r.permissionMenuRepo.Get(ctx, dto.GetTenantId(), dto.GetId())
			if err != nil {
				return nil, err
			}
			dto.Bind = &adminV1.Permission_MenuId{MenuId: menuID}
		}
		if hasApiResourceID && isApiPermission(dto.GetType()) {
			apiResourceID, err := r.permissionApiResourceRepo.Get(ctx, dto.GetTenantId(), dto.GetId())
			if err != nil {
				return nil, err
			}
			dto.Bind = &adminV1.Permission_ApiResourceId{ApiResourceId: apiResourceID}
		}
	}

	return &adminV1.ListPermissionResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *PermissionRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().Permission.Query().
		Where(permission.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, adminV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *PermissionRepo) Get(ctx context.Context, req *adminV1.GetPermissionRequest) (*adminV1.Permission, error) {
	if req == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Permission.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *adminV1.GetPermissionRequest_Id:
		whereCond = append(whereCond, permission.IDEQ(req.GetId()))
	case *adminV1.GetPermissionRequest_Code:
		whereCond = append(whereCond, permission.CodeEQ(req.GetCode()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	if hasPath("api_resource_id", req.GetViewMask()) && isApiPermission(dto.GetType()) {
		apiResourceID, err := r.permissionApiResourceRepo.Get(ctx, dto.GetTenantId(), dto.GetId())
		if err != nil {
			return nil, err
		}
		dto.Bind = &adminV1.Permission_ApiResourceId{ApiResourceId: apiResourceID}
	}
	if hasPath("menu_id", req.GetViewMask()) && isMenuPermission(dto.GetType()) {
		menuID, err := r.permissionMenuRepo.Get(ctx, dto.GetTenantId(), dto.GetId())
		if err != nil {
			return nil, err
		}
		dto.Bind = &adminV1.Permission_MenuId{MenuId: menuID}
	}

	return dto, err
}

func hasPath(path string, fieldMask *fieldmaskpb.FieldMask) bool {
	if fieldMask == nil {
		return true
	}
	for _, p := range fieldMask.GetPaths() {
		if path == p {
			return true
		}
	}
	return false
}

// Create 创建 Permission
func (r *PermissionRepo) Create(ctx context.Context, req *adminV1.CreatePermissionRequest) error {
	if req == nil || req.Data == nil {
		return adminV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newPermissionCreate(req.Data)

	var entity *ent.Permission
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert one data failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("insert data failed")
	}

	switch req.Data.Bind.(type) {
	case *adminV1.Permission_ApiResourceId:
		if err = r.permissionApiResourceRepo.AssignApi(ctx, req.Data.GetTenantId(), entity.ID, req.Data.GetApiResourceId()); err != nil {
			return err
		}
	case *adminV1.Permission_MenuId:
		if err = r.permissionMenuRepo.AssignMenu(ctx, req.Data.GetTenantId(), entity.ID, req.Data.GetMenuId()); err != nil {
			return err
		}
	}

	return nil
}

// BatchCreate 批量创建 Permission
func (r *PermissionRepo) BatchCreate(ctx context.Context, tenantID uint32, permissions []*adminV1.Permission) (err error) {
	if len(permissions) == 0 {
		return adminV1.ErrorBadRequest("invalid parameter")
	}

	var permissionCreates []*ent.PermissionCreate
	for _, perm := range permissions {
		pc := r.newPermissionCreate(perm)
		permissionCreates = append(permissionCreates, pc)
	}

	builder := r.entClient.Client().Permission.CreateBulk(permissionCreates...)

	var entities []*ent.Permission
	if entities, err = builder.Save(ctx); err != nil {
		r.log.Errorf("batch insert data failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("batch insert data failed")
	}

	apis := make(map[uint32]uint32)
	menus := make(map[uint32]uint32)
	for i, perm := range permissions {
		switch perm.Bind.(type) {
		case *adminV1.Permission_ApiResourceId:
			apis[entities[i].ID] = perm.GetApiResourceId()
		case *adminV1.Permission_MenuId:
			menus[entities[i].ID] = perm.GetMenuId()
		}
	}

	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf("transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf("transaction commit failed: %s", commitErr.Error())
			err = adminV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	if err = r.permissionApiResourceRepo.AssignApis(ctx, tx, tenantID, apis); err != nil {
		return err
	}

	if err = r.permissionMenuRepo.AssignMenus(ctx, tx, tenantID, menus); err != nil {
		return err
	}

	return nil
}

// newPermissionCreate 创建 Permission Create 构造器
func (r *PermissionRepo) newPermissionCreate(permission *adminV1.Permission) *ent.PermissionCreate {
	builder := r.entClient.Client().Permission.Create().
		SetNillableCode(permission.Code).
		SetName(permission.GetName()).
		SetNillablePath(permission.Path).
		SetNillableModule(permission.Module).
		SetNillableSortOrder(permission.SortOrder).
		SetNillableRemark(permission.Remark).
		SetNillableParentID(permission.ParentId).
		SetNillableType(r.typeConverter.ToEntity(permission.Type)).
		SetNillableStatus(r.statusConverter.ToEntity(permission.Status)).
		SetNillableCreatedBy(permission.CreatedBy).
		SetNillableCreatedAt(timeutil.TimestamppbToTime(permission.CreatedAt))

	if permission.TenantId == nil {
		builder.SetTenantID(permission.GetTenantId())
	}
	if permission.CreatedAt == nil {
		builder.SetCreatedAt(time.Now())
	}

	if permission.Id != nil {
		builder.SetID(permission.GetId())
	}

	return builder
}

// Update 更新 Permission
func (r *PermissionRepo) Update(ctx context.Context, req *adminV1.UpdatePermissionRequest) error {
	if req == nil || req.Data == nil {
		return adminV1.ErrorBadRequest("invalid parameter")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &adminV1.CreatePermissionRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Debug().Permission.UpdateOneID(req.GetId())
	perm, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *adminV1.Permission) {
			builder.
				SetNillableCode(req.Data.Code).
				SetNillableName(req.Data.Name).
				SetNillableModule(req.Data.Module).
				SetNillablePath(req.Data.Path).
				SetNillableSortOrder(req.Data.SortOrder).
				SetNillableRemark(req.Data.Remark).
				SetNillableParentID(req.Data.ParentId).
				SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
				SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetNillableUpdatedAt(timeutil.TimestamppbToTime(req.Data.UpdatedAt))

			if req.Data.UpdatedAt == nil {
				builder.SetUpdatedAt(time.Now())
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(permission.FieldID, req.GetId()))
		},
	)
	if err != nil {
		return err
	}

	switch req.Data.Bind.(type) {
	case *adminV1.Permission_ApiResourceId:
		if err = r.permissionApiResourceRepo.AssignApi(ctx, req.Data.GetTenantId(), perm.GetId(), req.Data.GetApiResourceId()); err != nil {
			return err
		}
		if err = r.permissionMenuRepo.Delete(ctx, perm.GetId()); err != nil {
			return err
		}
	case *adminV1.Permission_MenuId:
		if err = r.permissionMenuRepo.AssignMenu(ctx, req.Data.GetTenantId(), perm.GetId(), req.Data.GetMenuId()); err != nil {
			return err
		}
		if err = r.permissionApiResourceRepo.Delete(ctx, perm.GetId()); err != nil {
			return err
		}
	default:
		if err = r.permissionMenuRepo.Delete(ctx, perm.GetId()); err != nil {
			return err
		}
		if err = r.permissionApiResourceRepo.Delete(ctx, perm.GetId()); err != nil {
			return err
		}
	}

	return nil
}

// UpdateParentIDs 更新 Permission ParentID
func (r *PermissionRepo) UpdateParentIDs(ctx context.Context, parentIDs map[uint32]uint32) (err error) {
	if len(parentIDs) == 0 {
		return nil
	}

	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf("transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf("transaction commit failed: %s", commitErr.Error())
			err = adminV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	for permID, parentID := range parentIDs {
		builder := tx.Permission.Update().
			SetParentID(parentID).
			Where(permission.IDEQ(permID))

		if err = builder.Exec(ctx); err != nil {
			r.log.Errorf("update permission parent_id failed: %s", err.Error())
			return adminV1.ErrorInternalServerError("update permission parent_id failed")
		}
	}

	return nil
}

// UpdatePaths 更新 Permission Path
func (r *PermissionRepo) UpdatePaths(ctx context.Context, paths map[uint32]string) (err error) {
	if len(paths) == 0 {
		return nil
	}

	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf("transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf("transaction commit failed: %s", commitErr.Error())
			err = adminV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	for permID, path := range paths {
		builder := tx.Permission.Update().
			SetPath(path).
			Where(permission.IDEQ(permID))

		if err = builder.Exec(ctx); err != nil {
			r.log.Errorf("update permission path failed: %s", err.Error())
			return adminV1.ErrorInternalServerError("update permission path failed")
		}
	}

	return nil
}

// Delete 删除 Permission
func (r *PermissionRepo) Delete(ctx context.Context, req *adminV1.DeletePermissionRequest) error {
	if req == nil {
		return adminV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Permission.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(permission.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete permission failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("delete permission failed")
	}

	if err = r.permissionApiResourceRepo.Delete(ctx, req.GetId()); err != nil {
		return err
	}

	if err = r.permissionMenuRepo.Delete(ctx, req.GetId()); err != nil {
		return err
	}

	return nil
}

// Truncate 清空表数据
func (r *PermissionRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Permission.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate permission table: %s", err.Error())
		return adminV1.ErrorInternalServerError("truncate failed")
	}

	if err := r.permissionApiResourceRepo.Truncate(ctx); err != nil {
		return err
	}

	if err := r.permissionMenuRepo.Truncate(ctx); err != nil {
		return err
	}

	return nil
}

// CleanApiPermissions 清理API相关权限
func (r *PermissionRepo) CleanApiPermissions(ctx context.Context) error {
	if _, err := r.entClient.Client().Permission.Delete().
		Where(permission.TypeEQ(permission.TypeApi)).
		Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate permission table: %s", err.Error())
		return adminV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

// CleanDataPermissions 清理数据权限
func (r *PermissionRepo) CleanDataPermissions(ctx context.Context) error {
	if _, err := r.entClient.Client().Permission.Delete().
		Where(permission.TypeEQ(permission.TypeData)).
		Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate permission table: %s", err.Error())
		return adminV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

// CleanMenuPermissions 清理菜单相关权限
func (r *PermissionRepo) CleanMenuPermissions(ctx context.Context) error {
	if _, err := r.entClient.Client().Permission.Delete().
		Where(permission.TypeIn(
			permission.TypeCatalog,
			permission.TypeMenu,
			permission.TypeButton,
			permission.TypePage,
		)).
		Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate permission table: %s", err.Error())
		return adminV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

func (r *PermissionRepo) ListMenuPermissions(ctx context.Context) ([]*adminV1.Permission, error) {
	entities, err := r.entClient.Client().Permission.Query().
		Where(permission.TypeIn(
			permission.TypeCatalog,
			permission.TypeMenu,
			permission.TypeButton,
			permission.TypePage,
		)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query menu permissions failed: %s", err.Error())
		return nil, adminV1.ErrorInternalServerError("query menu permissions failed")
	}

	var dtos []*adminV1.Permission
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

func (r *PermissionRepo) ListApiPermissions(ctx context.Context) ([]*adminV1.Permission, error) {
	entities, err := r.entClient.Client().Permission.Query().
		Where(permission.TypeIn(
			permission.TypeApi,
		)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query api permissions failed: %s", err.Error())
		return nil, adminV1.ErrorInternalServerError("query api permissions failed")
	}

	var dtos []*adminV1.Permission
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		dtos = append(dtos, dto)
	}

	return dtos, nil
}
