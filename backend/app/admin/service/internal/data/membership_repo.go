package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-admin/app/admin/service/internal/data/ent"
	"go-wind-admin/app/admin/service/internal/data/ent/membership"

	userV1 "go-wind-admin/api/gen/go/user/service/v1"
)

type MembershipRepo struct {
	log *log.Helper

	entClient       *entCrud.EntClient[*ent.Client]
	mapper          *mapper.CopierMapper[userV1.Membership, ent.Membership]
	statusConverter *mapper.EnumTypeConverter[userV1.Membership_Status, membership.Status]

	membershipRoleRepo     *MembershipRoleRepo
	membershipPositionRepo *MembershipPositionRepo
	membershipOrgUnitRepo  *MembershipOrgUnitRepo
}

func NewMembershipRepo(
	ctx *bootstrap.Context,
	entClient *entCrud.EntClient[*ent.Client],
	membershipRoleRepo *MembershipRoleRepo,
	membershipPositionRepo *MembershipPositionRepo,
	membershipOrgUnitRepo *MembershipOrgUnitRepo,
) *MembershipRepo {
	repo := &MembershipRepo{
		log:       ctx.NewLoggerHelper("membership/repo/admin-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[userV1.Membership, ent.Membership](),
		statusConverter: mapper.NewEnumTypeConverter[userV1.Membership_Status, membership.Status](
			userV1.Membership_Status_name,
			userV1.Membership_Status_value,
		),
		membershipRoleRepo:     membershipRoleRepo,
		membershipPositionRepo: membershipPositionRepo,
		membershipOrgUnitRepo:  membershipOrgUnitRepo,
	}

	repo.init()

	return repo
}

func (r *MembershipRepo) init() {
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
}

// AssignTenantWithData 使用 Membership 数据为用户分配租户
func (r *MembershipRepo) AssignTenantWithData(
	ctx context.Context,
	operatorID *uint32,
	data *userV1.Membership,
) (err error) {
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

	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	return r.AssignTenant(
		ctx,
		tx,
		data.GetUserId(),
		data.GetTenantId(),
		operatorID,
		data.Status,
		startAt,
		endAt,
		assignedAt,
		data.AssignedBy,
		data.GetIsPrimary(),
	)
}

// AssignTenant 为用户分配租户
func (r *MembershipRepo) AssignTenant(
	ctx context.Context,
	tx *ent.Tx,
	userID, tenantID uint32,
	operatorID *uint32,
	status *userV1.Membership_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) error {
	var err error

	if _, err = tx.Membership.Delete().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		).
		Exec(ctx); err != nil {
		r.log.Errorf("delete old membership failed: %s", err.Error())
		return userV1.ErrorInternalServerError("delete old membership failed")
	}

	if _, err = r.upsertMembership(ctx, tx, &userV1.Membership{
		TenantId:   trans.Ptr(tenantID),
		UserId:     trans.Ptr(userID),
		Status:     status,
		CreatedBy:  operatorID,
		AssignedBy: assignedBy,
		AssignedAt: timeutil.TimeToTimestamppb(assignedAt),
		IsPrimary:  trans.Ptr(isPrimary),
		StartAt:    timeutil.TimeToTimestamppb(startAt),
		EndAt:      timeutil.TimeToTimestamppb(endAt),
	}); err != nil {
		r.log.Errorf("create membership failed: %s", err.Error())
		return userV1.ErrorInternalServerError("create membership failed")
	}

	return nil
}

func (r *MembershipRepo) AssignRolesWithTransaction(ctx context.Context,
	userID, tenantID uint32,
	roleIDs []uint32,
	operatorID *uint32,
	status *userV1.MembershipRole_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) (err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	var membershipID uint32
	membershipID, err = r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("get membership id failed")
	}

	// 调用 roleRepo 的核心方法（该方法接受 tx）
	if err = r.membershipRoleRepo.AssignRoles(
		ctx, tx,
		membershipID, tenantID,
		roleIDs, operatorID,
		status,
		startAt, endAt,
		assignedAt, assignedBy, isPrimary,
	); err != nil {
		return err
	}

	return nil
}

func (r *MembershipRepo) AssignPositionsWithTransaction(
	ctx context.Context,
	userID, tenantID uint32,
	positionIDs []uint32,
	operatorID *uint32,
	status *userV1.MembershipPosition_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) (err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	var membershipID uint32
	membershipID, err = r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("get membership id failed")
	}

	// 调用 positionRepo 的核心方法（该方法接受 tx）
	if err = r.membershipPositionRepo.AssignPositions(
		ctx, tx,
		membershipID, tenantID,
		positionIDs, operatorID,
		status,
		startAt, endAt,
		assignedAt, assignedBy, isPrimary,
	); err != nil {
		return err
	}

	return nil
}

func (r *MembershipRepo) AssignOrgUnitsWithTransaction(ctx context.Context,
	userID, tenantID uint32,
	orgUnitIDs []uint32,
	operatorID *uint32,
	status *userV1.MembershipOrgUnit_Status,
	startAt, endAt *time.Time,
	assignedAt *time.Time, assignedBy *uint32,
	isPrimary bool,
) (err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	var membershipID uint32
	membershipID, err = r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("get membership id failed")
	}

	// 调用 orgUnitRepo 的核心方法（该方法接受 tx）
	if err = r.membershipOrgUnitRepo.AssignOrgUnits(
		ctx, tx,
		membershipID, tenantID,
		orgUnitIDs, operatorID,
		status,
		startAt, endAt,
		assignedAt, assignedBy, isPrimary,
	); err != nil {
		return err
	}

	return nil
}

// SetUserOrgUnitID 设置用户的组织单元 ID
// 如果只是单一一个组织，可以直接在 Membership 表中设置 org_unit_id 字段。
func (r *MembershipRepo) SetUserOrgUnitID(ctx context.Context, userID, tenantID uint32, orgUnitID uint32) error {
	up := r.entClient.Client().Membership.
		Update().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		)

	if orgUnitID == 0 {
		if _, err := up.ClearOrgUnitID().Save(ctx); err != nil {
			r.log.Errorf("update membership org_unit_id failed: %s", err.Error())
			return userV1.ErrorInternalServerError("update membership org_unit_id failed")
		}
		return nil
	}

	if _, err := up.SetOrgUnitID(orgUnitID).Save(ctx); err != nil {
		r.log.Errorf("update membership org_unit_id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update membership org_unit_id failed")
	}
	return nil
}

// SetUserRoleID 设置用户的角色 ID
// 如果只是单一一个角色，可以直接在 Membership 表中设置 role_id 字段。
func (r *MembershipRepo) SetUserRoleID(ctx context.Context, userID, tenantID uint32, roleID uint32) error {
	up := r.entClient.Client().Membership.
		Update().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		)

	if roleID == 0 {
		if _, err := up.ClearRoleID().Save(ctx); err != nil {
			r.log.Errorf("update membership role_id failed: %s", err.Error())
			return userV1.ErrorInternalServerError("update membership role_id failed")
		}
		return nil
	}

	if _, err := up.SetRoleID(roleID).Save(ctx); err != nil {
		r.log.Errorf("update membership role_id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update membership role_id failed")
	}
	return nil
}

// SetUserPositionID 设置用户的职位 ID
// 如果只是单一个职位，可以直接在 Membership 表中设置 position_id 字段。
func (r *MembershipRepo) SetUserPositionID(ctx context.Context, userID, tenantID uint32, positionID uint32) error {
	up := r.entClient.Client().Membership.
		Update().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		)

	if positionID == 0 {
		if _, err := up.ClearPositionID().Save(ctx); err != nil {
			r.log.Errorf("update membership position_id failed: %s", err.Error())
			return userV1.ErrorInternalServerError("update membership position_id failed")
		}
		return nil
	}

	if _, err := up.SetPositionID(positionID).Save(ctx); err != nil {
		r.log.Errorf("update membership position_id failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update membership position_id failed")
	}
	return nil
}

// SetUserStatus 设置用户的状态
func (r *MembershipRepo) SetUserStatus(ctx context.Context, userID, tenantID uint32, status *userV1.Membership_Status) error {
	up := r.entClient.Client().Membership.
		Update().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		)

	if status == nil {
		if _, err := up.ClearStatus().Save(ctx); err != nil {
			r.log.Errorf("update membership status failed: %s", err.Error())
			return userV1.ErrorInternalServerError("update membership status failed")
		}
		return nil
	}

	if _, err := up.SetStatus(*r.statusConverter.ToEntity(status)).Save(ctx); err != nil {
		r.log.Errorf("update membership status failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update membership status failed")
	}
	return nil
}

// SetUserEndAt 设置用户的结束时间
func (r *MembershipRepo) SetUserEndAt(ctx context.Context, userID, tenantID uint32, endAt *time.Time) error {
	up := r.entClient.Client().Membership.
		Update().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
		)

	if endAt == nil {
		if _, err := up.ClearEndAt().Save(ctx); err != nil {
			r.log.Errorf("update membership end_at failed: %s", err.Error())
			return userV1.ErrorInternalServerError("update membership end_at failed")
		}
		return nil
	}

	if _, err := up.SetEndAt(*endAt).Save(ctx); err != nil {
		r.log.Errorf("update membership end_at failed: %s", err.Error())
		return userV1.ErrorInternalServerError("update membership end_at failed")
	}
	return nil
}

// GetMembershipID 获取 Membership ID
func (r *MembershipRepo) GetMembershipID(ctx context.Context, userID, tenantID uint32) (membershipID uint32, err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return uint32(0), userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	return r.queryMembershipID(ctx, tx, userID, tenantID)
}

func (r *MembershipRepo) GetMembership(ctx context.Context, userID, tenantID uint32) (*userV1.Membership, error) {
	now := time.Now()
	builder := r.entClient.Client().Membership.Query()
	builder.Where(
		membership.TenantIDEQ(tenantID),
		membership.UserIDEQ(userID),
		membership.Or(
			membership.EndAtIsNil(),
			membership.EndAtGT(now),
		),
	)

	entity, err := builder.Only(ctx)
	if err != nil {
		r.log.Errorf("get membership failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("get membership failed")
	}

	dto := r.mapper.ToDTO(entity)

	return dto, nil
}

// ListMembershipRoleIDs 获取 Membership 关联的角色 ID 列表
func (r *MembershipRepo) ListMembershipRoleIDs(ctx context.Context, userID, tenantID uint32) (roleIDs []uint32, err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	var membershipID uint32
	membershipID, err = r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("get membership id failed")
	}

	return r.membershipRoleRepo.ListRoleIDs(ctx, membershipID, tenantID, false)
}

// ListMembershipOrgUnitIDs 获取 Membership 关联的组织单元 ID 列表
func (r *MembershipRepo) ListMembershipOrgUnitIDs(ctx context.Context, userID, tenantID uint32) (orgUnitIDs []uint32, err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	membershipID, err := r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("get membership id failed")
	}

	return r.membershipOrgUnitRepo.ListOrgUnitIDs(ctx, membershipID, tenantID, false)
}

// ListMembershipPositionIDs 获取 Membership 关联的职位 ID 列表
func (r *MembershipRepo) ListMembershipPositionIDs(ctx context.Context, userID, tenantID uint32) (positionIDs []uint32, err error) {
	var tx *ent.Tx
	tx, err = r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("start transaction failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("start transaction failed")
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
			err = userV1.ErrorInternalServerError("transaction commit failed")
		}
	}()

	membershipID, err := r.queryMembershipID(ctx, tx, userID, tenantID)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("get membership id failed")
	}

	return r.membershipPositionRepo.ListPositionIDs(ctx, membershipID, tenantID, false)
}

// ListMembershipAllIDs 获取 Membership 关联的所有角色、职位、组织单元 ID 列表
func (r *MembershipRepo) ListMembershipAllIDs(ctx context.Context, userID, tenantID uint32) (roleIDs []uint32, positionIDs []uint32, orgUnitIDs []uint32, err error) {
	now := time.Now()

	builder := r.entClient.Client().Membership.Query()
	builder.Where(
		membership.TenantIDEQ(tenantID),
		membership.UserIDEQ(userID),
		membership.Or(
			membership.EndAtIsNil(),
			membership.EndAtGT(now),
		),
	)
	builder.Select(
		membership.FieldID,
	)

	ms, err := builder.Only(ctx)
	if err != nil {
		r.log.Errorf("get membership failed: %s", err.Error())
		return nil, nil, nil, userV1.ErrorInternalServerError("get membership failed")
	}
	if ms == nil {
		r.log.Errorf("membership not found for user %d in tenant %d", userID, tenantID)
		return nil, nil, nil, userV1.ErrorNotFound("membership not found")
	}

	membershipID := ms.ID

	roleIDs, err = r.membershipRoleRepo.ListRoleIDs(ctx, membershipID, tenantID, false)
	if err != nil {
		return nil, nil, nil, err
	}

	positionIDs, err = r.membershipPositionRepo.ListPositionIDs(ctx, membershipID, tenantID, false)
	if err != nil {
		return nil, nil, nil, err
	}

	orgUnitIDs, err = r.membershipOrgUnitRepo.ListOrgUnitIDs(ctx, membershipID, tenantID, false)
	if err != nil {
		return nil, nil, nil, err
	}

	return
}

// createMembership 创建 Membership 记录
func (r *MembershipRepo) createMembership(ctx context.Context, tx *ent.Tx, data *userV1.Membership) (*ent.Membership, error) {
	now := time.Now()

	if data.StartAt == nil {
		data.StartAt = timeutil.TimeToTimestamppb(&now)
	}

	cr := tx.Membership.
		Create().
		SetTenantID(data.GetTenantId()).
		SetUserID(data.GetUserId()).
		SetNillableRoleID(data.RoleId).
		SetNillablePositionID(data.PositionId).
		SetNillableOrgUnitID(data.OrgUnitId).
		SetNillableStatus(r.statusConverter.ToEntity(data.Status)).
		SetNillableAssignedBy(data.AssignedBy).
		SetNillableAssignedAt(timeutil.TimestamppbToTime(data.AssignedAt)).
		SetNillableIsPrimary(data.IsPrimary).
		SetNillableStartAt(timeutil.TimestamppbToTime(data.StartAt)).
		SetNillableEndAt(timeutil.TimestamppbToTime(data.EndAt)).
		SetNillableCreatedBy(data.CreatedBy).
		SetCreatedAt(now)

	if entity, err := cr.Save(ctx); err != nil {
		r.log.Errorf("create membership failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("create membership failed")
	} else {
		return entity, err
	}
}

// upsertMembership 更新或插入 Membership 记录
func (r *MembershipRepo) upsertMembership(ctx context.Context, tx *ent.Tx, data *userV1.Membership) (*ent.Membership, error) {
	now := time.Now()

	if data.StartAt == nil {
		data.StartAt = timeutil.TimeToTimestamppb(&now)
	}

	builder := tx.Membership.Create()

	builder.
		SetTenantID(data.GetTenantId()).
		SetUserID(data.GetUserId()).
		SetNillableRoleID(data.RoleId).
		SetNillablePositionID(data.PositionId).
		SetNillableOrgUnitID(data.OrgUnitId).
		SetNillableStatus(r.statusConverter.ToEntity(data.Status)).
		SetNillableAssignedBy(data.AssignedBy).
		SetNillableAssignedAt(timeutil.TimestamppbToTime(data.AssignedAt)).
		SetNillableIsPrimary(data.IsPrimary).
		SetNillableStartAt(timeutil.TimestamppbToTime(data.StartAt)).
		SetNillableEndAt(timeutil.TimestamppbToTime(data.EndAt)).
		SetNillableCreatedBy(data.CreatedBy).
		SetCreatedAt(now).
		OnConflictColumns(
			membership.FieldTenantID,
			membership.FieldUserID,
		).
		UpdateNewValues().
		SetUpdatedAt(now).
		SetUpdatedBy(data.GetUpdatedBy())

	if entity, err := builder.Save(ctx); err != nil {
		r.log.Errorf("upsert membership failed: %s", err.Error())
		return nil, userV1.ErrorInternalServerError("upsert membership failed")
	} else {
		return entity, err
	}
}

// queryMembershipID 查询 Membership ID
func (r *MembershipRepo) queryMembershipID(
	ctx context.Context, tx *ent.Tx,
	userID, tenantID uint32,
) (uint32, error) {
	now := time.Now()
	membershipID, err := tx.Membership.Query().
		Where(
			membership.TenantIDEQ(tenantID),
			membership.UserIDEQ(userID),
			membership.Or(
				membership.EndAtIsNil(),
				membership.EndAtGT(now),
			),
		).
		OnlyID(ctx)
	if err != nil {
		r.log.Errorf("get membership id failed: %s", err.Error())
		return 0, userV1.ErrorInternalServerError("get membership id failed")
	}
	return membershipID, nil
}
