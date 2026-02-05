package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ExchangeAccount holds the schema definition for the ExchangeAccount entity.
type ExchangeAccount struct {
	ent.Schema
}

func (ExchangeAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "trading_exchange_accounts",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("交易账号表"),
	}
}

// Mixin of the ExchangeAccount.
func (ExchangeAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Fields of the ExchangeAccount.
func (ExchangeAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("nickname").
			Comment("账号昵称").
			MaxLen(300).
			NotEmpty(),

		field.String("exchange_name").
			Comment("交易所名称").
			MaxLen(300).
			NotEmpty(),

		field.String("origin_account").
			Comment("原始账号").
			MaxLen(300).
			NotEmpty(),

		field.String("api_key").
			Comment("API密钥").
			MaxLen(600).
			NotEmpty(),

		field.String("secret_key").
			Comment("密钥（加密存储）").
			MaxLen(2000).
			NotEmpty().
			Sensitive(),

		field.String("pass_key").
			Comment("密码密钥").
			MaxLen(600).
			Default("").
			Optional().
			Nillable().
			Sensitive(),

		field.String("broker_id").
			Comment("经纪商ID").
			MaxLen(128).
			Default("").
			Optional().
			Nillable(),

		field.String("remark").
			Comment("备注").
			MaxLen(256).
			Default("").
			Optional().
			Nillable(),

		field.String("server_ips").
			Comment("绑定的托管者IP列表（逗号分隔）").
			Default("").
			Optional().
			Nillable(),

		field.Float("special_req_limit").
			Comment("特殊限频").
			Default(0).
			Optional().
			Nillable(),

		field.Int8("account_type").
			Comment("账号类型：1=自建，2=平台").
			Default(1),

		field.Int64("apply_time").
			Comment("申请时间").
			Default(0).
			Optional().
			Nillable(),

		field.Bool("is_combined").
			Comment("是否参与组合账号").
			Default(false),

		field.Bool("is_multi").
			Comment("是否是组合账号").
			Default(false),

		field.JSON("account_ids", []string{}).
			Comment("组合账号ID列表").
			Optional(),

		field.Uint32("mother_id").
			Comment("母账号ID").
			Default(0).
			Optional().
			Nillable(),
	}
}

// Edges of the ExchangeAccount.
func (ExchangeAccount) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Indexes of the ExchangeAccount.
func (ExchangeAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exchange_name"),
		index.Fields("origin_account"),
		index.Fields("api_key").Unique(),
		index.Fields("account_type"),
		index.Fields("is_multi"),
	}
}
