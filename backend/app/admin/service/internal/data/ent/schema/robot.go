package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Robot holds the schema definition for the Robot entity.
type Robot struct {
	ent.Schema
}

func (Robot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "trading_robots",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("机器人表"),
	}
}

// Fields of the Robot.
func (Robot) Fields() []ent.Field {
	return []ent.Field{
		field.String("rid").
			Comment("机器人ID").
			NotEmpty().
			Immutable(),

		field.String("nickname").
			Comment("昵称").
			Optional().
			Default(""),

		field.String("exchange").
			Comment("交易所").
			Optional().
			Default(""),

		field.String("version").
			Comment("版本").
			Optional().
			Default(""),

		field.String("status").
			Comment("机器人状态").
			Optional().
			Default(""),

		field.Float("balance").
			Comment("当前资金").
			Default(0.0),

		field.Float("init_balance").
			Comment("初始资金").
			Default(0.0),

		field.Time("registered_at").
			Comment("注册时间").
			Optional().
			Nillable(),

		field.Time("last_heartbeat").
			Comment("最后心跳时间").
			Optional().
			Nillable(),

		field.Uint32("server_id").
			Comment("关联服务器ID").
			Optional().
			Default(0),

		field.Uint32("exchange_account_id").
			Comment("关联交易账号ID").
			Optional().
			Default(0),
	}
}

// Mixin of the Robot.
func (Robot) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.OperatorID{},
		mixin.TimeAt{},
		mixin.Remark{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the Robot.
func (Robot) Indexes() []ent.Index {
	return []ent.Index{
		// 在租户范围内保证 rid 唯一
		index.Fields("tenant_id", "rid").Unique().StorageKey("idx_app_robot_tenant_rid"),

		// 按租户 + 状态，用于按状态查询
		index.Fields("tenant_id", "status").StorageKey("idx_app_robot_tenant_status"),

		// 按租户 + 创建时间，用于时间区间查询与分页
		index.Fields("tenant_id", "created_at").StorageKey("idx_app_robot_tenant_created_at"),
	}
}
