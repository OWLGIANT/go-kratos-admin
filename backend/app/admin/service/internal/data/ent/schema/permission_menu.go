package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// PermissionMenu holds the schema definition for the PermissionMenu entity.
type PermissionMenu struct {
	ent.Schema
}

func (PermissionMenu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "sys_permission_menus",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("权限点 - 前端菜单多对多关联表"),
	}
}

// Fields of the PermissionMenu.
func (PermissionMenu) Fields() []ent.Field {
	return []ent.Field{

		field.Uint32("menu_id").
			Comment("菜单ID（关联sys_menus.id）").
			Nillable(),

		field.Uint32("permission_id").
			Comment("权限ID（关联sys_permissions.id）").
			Nillable(),
	}
}

// Mixin of the PermissionMenu.
func (PermissionMenu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID{},
	}
}

// Indexes of the PermissionMenu.
func (PermissionMenu) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "permission_id").
			Unique().
			StorageKey("uix_perm_menu_tenant"),
	}
}
