package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/permissionmenu"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
)

type PermissionMenuRepo struct {
	log       *log.Helper
	entClient *entCrud.EntClient[*ent.Client]
}

func NewPermissionMenuRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *PermissionMenuRepo {
	return &PermissionMenuRepo{
		log:       ctx.NewLoggerHelper("permission-menu/repo/admin-service"),
		entClient: entClient,
	}
}

// CleanMenus 清理权限的所有菜单
func (r *PermissionMenuRepo) CleanMenus(
	ctx context.Context,
	tx *ent.Tx,
	tenantID uint32,
	permissionIDs []uint32,
) error {
	if _, err := tx.PermissionMenu.Delete().
		Where(
			permissionmenu.PermissionIDIn(permissionIDs...),
			permissionmenu.TenantIDEQ(tenantID),
		).
		Exec(ctx); err != nil {
		err = entCrud.Rollback(tx, err)
		r.log.Errorf("delete old permission menus failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("delete old permission menus failed")
	}
	return nil
}

// AssignMenus 给权限分配菜单
func (r *PermissionMenuRepo) AssignMenus(ctx context.Context, tx *ent.Tx, tenantID uint32, menus map[uint32]uint32) error {
	if len(menus) == 0 {
		return nil
	}

	for permissionID, menuID := range menus {
		pm := tx.PermissionMenu.
			Create().
			SetPermissionID(permissionID).
			SetMenuID(menuID).
			SetTenantID(tenantID).
			OnConflict().
			UpdateNewValues()
		if err := pm.Exec(ctx); err != nil {
			err = entCrud.Rollback(tx, err)
			r.log.Errorf("assign permission menus failed: %s", err.Error())
			return adminV1.ErrorInternalServerError("assign permission menus failed")
		}
	}

	return nil
}

// ListMenuIDs 列出权限关联的菜单ID列表
func (r *PermissionMenuRepo) ListMenuIDs(ctx context.Context, tenantID uint32, permissionIDs []uint32) ([]uint32, error) {
	q := r.entClient.Client().PermissionMenu.
		Query().
		Where(
			permissionmenu.PermissionIDIn(permissionIDs...),
			permissionmenu.TenantIDEQ(tenantID),
		)

	intIDs, err := q.
		Select(permissionmenu.FieldMenuID).
		Ints(ctx)
	if err != nil {
		r.log.Errorf("list permission menus by permission id failed: %s", err.Error())
		return nil, adminV1.ErrorInternalServerError("list permission menus by permission id failed")
	}

	ids := make([]uint32, len(intIDs))
	for i, v := range intIDs {
		ids[i] = uint32(v)
	}
	return ids, nil
}
