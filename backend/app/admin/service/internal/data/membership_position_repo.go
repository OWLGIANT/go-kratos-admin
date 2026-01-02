package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/membershipposition"

	userV1 "go-wind-admin/api/gen/go/user/service/v1"
)

type MembershipPositionRepo struct {
	log             *log.Helper
	entClient       *entCrud.EntClient[*ent.Client]
	statusConverter *mapper.EnumTypeConverter[userV1.MembershipPosition_Status, membershipposition.Status]
}

func NewMembershipPositionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *MembershipPositionRepo {
	return &MembershipPositionRepo{
		log:       ctx.NewLoggerHelper("membership-position/repo/admin-service"),
		entClient: entClient,
		statusConverter: mapper.NewEnumTypeConverter[userV1.MembershipPosition_Status, membershipposition.Status](
			userV1.MembershipPosition_Status_name,
			userV1.MembershipPosition_Status_value,
		),
	}
}

// CleanPositions 删除会员在某租户下的所有职位关联
func (r *MembershipPositionRepo) CleanPositions(
	ctx context.Context,
	tx *ent.Tx,
	membershipID, tenantID uint32,
) error {
	if _, err := tx.MembershipPosition.Delete().
		Where(
			membershipposition.MembershipIDEQ(membershipID),
			membershipposition.TenantIDEQ(tenantID),
		).
		Exec(ctx); err != nil {
		err = entCrud.Rollback(tx, err)
		r.log.Errorf("delete old membership positions failed: %s", err.Error())
		return userV1.ErrorInternalServerError("delete old membership positions failed")
	}
	return nil
}

func (r *MembershipPositionRepo) AssignPositionWithData(ctx context.Context, operatorID *uint32, data *userV1.MembershipPosition) error {
	var startAt *time.Time
	var endAt *time.Time
	var assignedAt *time.Time
	if data.StartAt != nil {
		startAt = trans.Ptr(data.StartAt.AsTime())
	}
	if data.EndAt != nil {
		endAt = trans.Ptr(data.EndAt.AsTime())
	}
	if data.AssignedAt != nil {
		assignedAt = trans.Ptr(data.AssignedAt.AsTime())
	}

	tx, err := r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("start transaction failed")
	}

	return r.AssignPositions(
		ctx,
		tx,
		data.GetMembershipId(),
		data.GetTenantId(),
		[]uint32{data.GetPositionId()},
		operatorID,
		data.Status,
		startAt,
		endAt,
		assignedAt,
		data.AssignedBy,
		data.GetIsPrimary(),
	)
}

// AssignPositions 分配岗位给用户
func (r *MembershipPositionRepo) AssignPositions(
	ctx context.Context,
	tx *ent.Tx,
	membershipID, tenantID uint32,
	positionIDs []uint32, operatorID *uint32,
	status *userV1.MembershipPosition_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) error {
	var err error

	// 删除该用户的所有旧关联
	if err = r.CleanPositions(ctx, tx, membershipID, tenantID); err != nil {
		return userV1.ErrorInternalServerError("clean old membership positions failed")
	}

	// 如果没有分配任何，则直接提交事务返回
	if len(positionIDs) == 0 {
		// 提交事务
		if err = tx.Commit(); err != nil {
			r.log.Errorf("commit transaction failed: %s", err.Error())
			return userV1.ErrorInternalServerError("commit transaction failed")
		}
		return nil
	}

	now := time.Now()

	if startAt == nil {
		startAt = &now
	}

	var membershipPositionCreates []*ent.MembershipPositionCreate
	for _, id := range positionIDs {
		rm := tx.MembershipPosition.
			Create().
			SetMembershipID(membershipID).
			SetPositionID(id).
			SetNillableStatus(r.statusConverter.ToEntity(status)).
			SetNillableCreatedBy(operatorID).
			SetNillableAssignedBy(assignedBy).
			SetNillableAssignedAt(assignedAt).
			SetTenantID(tenantID).
			SetIsPrimary(isPrimary).
			SetCreatedAt(now).
			SetNillableStartAt(startAt).
			SetNillableEndAt(endAt)
		membershipPositionCreates = append(membershipPositionCreates, rm)
	}

	_, err = tx.MembershipPosition.CreateBulk(membershipPositionCreates...).Save(ctx)
	if err != nil {
		err = entCrud.Rollback(tx, err)
		r.log.Errorf("assign positions to membership failed: %s", err.Error())
		return userV1.ErrorInternalServerError("assign positions to membership failed")
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		r.log.Errorf("commit transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("commit transaction failed")
	}

	return nil
}

// ListPositionIDs 获取用户的岗位ID列表
func (r *MembershipPositionRepo) ListPositionIDs(ctx context.Context, membershipID, tenantID uint32, excludeExpired bool) ([]uint32, error) {
	q := r.entClient.Client().MembershipPosition.Query().
		Where(
			membershipposition.MembershipIDEQ(membershipID),
			membershipposition.TenantIDEQ(tenantID),
		)

	if excludeExpired {
		now := time.Now()
		q = q.Where(
			membershipposition.Or(
				membershipposition.EndAtIsNil(),
				membershipposition.EndAtGT(now),
			),
		)
	}

	intIDs, err := q.
		Select(membershipposition.FieldPositionID).
		Ints(ctx)
	if err != nil {
		r.log.Errorf("query position ids by membership id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query position ids by membership id failed")
	}
	ids := make([]uint32, len(intIDs))
	for i, v := range intIDs {
		ids[i] = uint32(v)
	}
	return ids, nil
}

// RemovePositions 从用户移除岗位
func (r *MembershipPositionRepo) RemovePositions(ctx context.Context, membershipID, tenantID uint32, positionIDs []uint32) error {
	_, err := r.entClient.Client().MembershipPosition.Delete().
		Where(
			membershipposition.And(
				membershipposition.MembershipIDEQ(membershipID),
				membershipposition.TenantIDEQ(tenantID),
				membershipposition.PositionIDIn(positionIDs...),
			),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("remove positions from membership failed: %s", err.Error())
		return userV1.ErrorInternalServerError("remove positions from membership failed")
	}
	return nil
}
