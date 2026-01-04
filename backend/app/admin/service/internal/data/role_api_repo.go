package data

import (
	"context"
	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/roleapi"

	userV1 "go-wind-admin/api/gen/go/user/service/v1"
)

type RoleApiRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewRoleApiRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *RoleApiRepo {
	return &RoleApiRepo{
		log:       ctx.NewLoggerHelper("role-api/repo/admin-service"),
		entClient: entClient,
	}
}

// AssignApis 给角色分配API
func (r *RoleApiRepo) AssignApis(ctx context.Context, roleId uint32, apiIds []uint32, operatorId uint32) (err error) {
	// 开启事务
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

	// 删除该角色的所有旧关联
	if _, err = tx.RoleApi.Delete().
		Where(roleapi.RoleID(roleId)).
		Exec(ctx); err != nil {
		r.log.Errorf("delete old role apis failed: %s", err.Error())
		return userV1.ErrorInternalServerError("delete old role apis failed")
	}

	// 如果没有分配任何，则直接提交事务返回
	if len(apiIds) == 0 {
		return nil
	}

	var roleApis []*ent.RoleApiCreate
	for _, apiId := range apiIds {
		rm := tx.RoleApi.
			Create().
			SetRoleID(roleId).
			SetAPIID(apiId).
			SetCreatedBy(operatorId).
			SetCreatedAt(time.Now())
		roleApis = append(roleApis, rm)
	}

	_, err = tx.RoleApi.CreateBulk(roleApis...).Save(ctx)
	if err != nil {
		r.log.Errorf("assign apis to role failed: %s", err.Error())
		return userV1.ErrorInternalServerError("assign apis to role failed")
	}

	return nil
}

// ListApiIdsByRoleId 获取角色分配的API ID列表
func (r *RoleApiRepo) ListApiIdsByRoleId(ctx context.Context, roleId uint32) ([]uint32, error) {
	intIDs, err := r.entClient.Client().RoleApi.Query().
		Where(roleapi.RoleIDEQ(roleId)).
		Select(roleapi.FieldAPIID).
		Ints(ctx)
	if err != nil {
		r.log.Errorf("query api ids by role id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query api ids by role id failed")
	}

	ids := make([]uint32, len(intIDs))
	for i, v := range intIDs {
		ids[i] = uint32(v)
	}
	return ids, nil
}

// RemoveApis 从角色移除API
func (r *RoleApiRepo) RemoveApis(ctx context.Context, roleId uint32, apiIds []uint32) error {
	_, err := r.entClient.Client().RoleApi.Delete().
		Where(
			roleapi.And(
				roleapi.RoleIDEQ(roleId),
				roleapi.APIIDIn(apiIds...),
			),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("remove apis from role failed: %s", err.Error())
		return userV1.ErrorInternalServerError("remove apis from role failed")
	}
	return nil
}
