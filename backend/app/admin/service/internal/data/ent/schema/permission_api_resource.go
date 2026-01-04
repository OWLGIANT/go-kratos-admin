package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// PermissionApiResource holds the schema definition for the PermissionApiResource entity.
type PermissionApiResource struct {
	ent.Schema
}

func (PermissionApiResource) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "sys_permission_api_resources",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("权限点 - API接口多对多关联表"),
	}
}

// Fields of the PermissionApiResource.
func (PermissionApiResource) Fields() []ent.Field {
	return []ent.Field{

		field.Uint32("api_resource_id").
			Comment("API资源ID（关联sys_api_resources.id）").
			Nillable(),

		field.Uint32("permission_id").
			Comment("权限ID（关联sys_permissions.id）").
			Nillable(),
	}
}

// Mixin of the PermissionApiResource.
func (PermissionApiResource) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID{},
	}
}

// Indexes of the PermissionApiResource.
func (PermissionApiResource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "permission_id").
			Unique().
			StorageKey("uix_perm_api_tenant"),
	}
}
