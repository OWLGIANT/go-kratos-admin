package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Server holds the schema definition for the Server entity.
type Server struct {
	ent.Schema
}

func (Server) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "trading_servers",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("托管者（服务器）表"),
	}
}

// Mixin of the Server.
func (Server) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Fields of the Server.
func (Server) Fields() []ent.Field {
	return []ent.Field{
		field.String("nickname").
			Comment("托管者昵称").
			MaxLen(32).
			NotEmpty(),

		field.String("ip").
			Comment("外网IP").
			MaxLen(32).
			NotEmpty(),

		field.String("inner_ip").
			Comment("内网IP").
			MaxLen(32).
			NotEmpty(),

		field.String("port").
			Comment("端口").
			MaxLen(8).
			NotEmpty(),

		field.String("machine_id").
			Comment("机器ID").
			MaxLen(64).
			Default("").
			Optional().
			Nillable(),

		field.String("remark").
			Comment("备注").
			MaxLen(256).
			Default("").
			Optional().
			Nillable(),

		field.String("vpc_id").
			Comment("VPC ID").
			MaxLen(50).
			Default("NOT_COMMON").
			Optional().
			Nillable(),

		field.String("instance_id").
			Comment("实例ID").
			MaxLen(64).
			Default("").
			Optional().
			Nillable(),

		field.Int8("type").
			Comment("类型：1=自建，2=大后台").
			Default(1),

		// 服务器状态信息（JSON字段）
		field.JSON("server_info", map[string]interface{}{}).
			Comment("服务器状态信息").
			Optional(),
	}
}

// Edges of the Server.
func (Server) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Indexes of the Server.
func (Server) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ip").Unique(),
		index.Fields("inner_ip"),
		index.Fields("type"),
		index.Fields("vpc_id"),
	}
}
