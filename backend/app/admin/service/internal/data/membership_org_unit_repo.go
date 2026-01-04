package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/membershiporgunit"

	userV1 "go-wind-admin/api/gen/go/user/service/v1"
)

type MembershipOrgUnitRepo struct {
	log *log.Helper

	entClient       *entCrud.EntClient[*ent.Client]
	statusConverter *mapper.EnumTypeConverter[userV1.MembershipOrgUnit_Status, membershiporgunit.Status]
}

func NewMembershipOrgUnitRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *MembershipOrgUnitRepo {
	return &MembershipOrgUnitRepo{
		log:       ctx.NewLoggerHelper("membership-org-unit/repo/admin-service"),
		entClient: entClient,
		statusConverter: mapper.NewEnumTypeConverter[userV1.MembershipOrgUnit_Status, membershiporgunit.Status](
			userV1.MembershipOrgUnit_Status_name,
			userV1.MembershipOrgUnit_Status_value,
		),
	}
}

// CleanOrgUnits 清理会员组织单元关联
func (r *MembershipOrgUnitRepo) CleanOrgUnits(
	ctx context.Context,
	tx *ent.Tx,
	membershipID, tenantID uint32,
) error {
	if _, err := tx.MembershipOrgUnit.Delete().
		Where(
			membershiporgunit.MembershipIDEQ(membershipID),
			membershiporgunit.TenantIDEQ(tenantID),
		).
		Exec(ctx); err != nil {
		r.log.Errorf("delete old membership orgUnits failed: %s", err.Error())
		return userV1.ErrorInternalServerError("delete old membership orgUnits failed")
	}
	return nil
}

// AssignOrgUnits 分配组织单元给会员
func (r *MembershipOrgUnitRepo) AssignOrgUnits(
	ctx context.Context,
	tx *ent.Tx,
	membershipID, tenantID uint32,
	orgUnitIDs []uint32, operatorID *uint32,
	status *userV1.MembershipOrgUnit_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) error {
	var err error

	// 删除该角色的所有旧关联
	if err = r.CleanOrgUnits(ctx, tx, membershipID, tenantID); err != nil {
		return userV1.ErrorInternalServerError("clean old membership orgUnits failed")
	}

	// 如果没有分配任何，则直接提交事务返回
	if len(orgUnitIDs) == 0 {
		return nil
	}

	now := time.Now()

	if startAt == nil {
		startAt = &now
	}

	var membershipOrgUnitCreates []*ent.MembershipOrgUnitCreate
	for _, id := range orgUnitIDs {
		rm := tx.MembershipOrgUnit.
			Create().
			SetMembershipID(membershipID).
			SetOrgUnitID(id).
			SetNillableStatus(r.statusConverter.ToEntity(status)).
			SetNillableCreatedBy(operatorID).
			SetNillableAssignedBy(assignedBy).
			SetNillableAssignedAt(assignedAt).
			SetTenantID(tenantID).
			SetIsPrimary(isPrimary).
			SetCreatedAt(now).
			SetNillableStartAt(startAt).
			SetNillableEndAt(endAt)
		membershipOrgUnitCreates = append(membershipOrgUnitCreates, rm)
	}

	_, err = tx.MembershipOrgUnit.CreateBulk(membershipOrgUnitCreates...).Save(ctx)
	if err != nil {
		r.log.Errorf("assign organizations to role failed: %s", err.Error())
		return userV1.ErrorInternalServerError("assign organizations to role failed")
	}

	return nil
}

// ListOrgUnitIDs 列出角色关联的组织单元ID列表
func (r *MembershipOrgUnitRepo) ListOrgUnitIDs(ctx context.Context, membershipID, tenantID uint32, excludeExpired bool) ([]uint32, error) {
	q := r.entClient.Client().MembershipOrgUnit.Query().
		Where(
			membershiporgunit.MembershipIDEQ(membershipID),
			membershiporgunit.TenantIDEQ(tenantID),
		)

	if excludeExpired {
		now := time.Now()
		q = q.Where(
			membershiporgunit.Or(
				membershiporgunit.EndAtIsNil(),
				membershiporgunit.EndAtGT(now),
			),
		)
	}

	intIDs, err := q.
		Select(membershiporgunit.FieldOrgUnitID).
		Ints(ctx)
	if err != nil {
		r.log.Errorf("query orgUnit ids by membership id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("query orgUnit ids by membership id failed")
	}
	ids := make([]uint32, len(intIDs))
	for i, v := range intIDs {
		ids[i] = uint32(v)
	}
	return ids, nil
}

// RemoveOrgUnits 删除会员的组织单元关联
func (r *MembershipOrgUnitRepo) RemoveOrgUnits(ctx context.Context, membershipID, tenantID uint32, ids []uint32) error {
	_, err := r.entClient.Client().MembershipOrgUnit.Delete().
		Where(
			membershiporgunit.And(
				membershiporgunit.MembershipIDEQ(membershipID),
				membershiporgunit.TenantIDEQ(tenantID),
				membershiporgunit.OrgUnitIDIn(ids...),
			),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("remove orgUnits failed: %s", err.Error())
		return userV1.ErrorInternalServerError("remove orgUnits failed")
	}
	return nil
}
