-- Description: 初始化默认用户、角色、菜单和API资源数据
-- Note: 需要有表结构之后再执行此脚本。另，请确保在执行此脚本前已备份相关数据，以防数据丢失。

BEGIN;

SET LOCAL search_path = public, pg_catalog;

-- 一次性清理相关表并重置自增（包含外键依赖）
TRUNCATE TABLE public.sys_user_credentials,
               public.sys_users,
               public.sys_user_org_units,
               public.sys_user_positions,
               public.sys_user_roles,
               public.sys_tenants,
               public.sys_org_units,
               public.sys_positions,
               public.sys_roles,
               public.sys_role_permissions,
               public.sys_menus,
               public.sys_apis,
               public.sys_permissions,
               public.sys_permission_groups,
               public.sys_permission_apis,
               public.sys_permission_menus,
               public.sys_permission_policies,
               public.sys_permissions,
               public.sys_memberships,
               public.sys_membership_org_units,
               public.sys_membership_positions,
               public.sys_membership_roles
RESTART IDENTITY CASCADE;

-- 默认的用户
INSERT INTO public.sys_users (id, tenant_id, username, nickname, realname, email, gender, created_at)
VALUES
    -- 1. 系统管理员（ADMIN）
    (1, 0, 'admin', '鹳狸猿', '喵个咪', 'admin@gmail.com', 'MALE', now()),
;
SELECT setval('sys_users_id_seq', (SELECT MAX(id) FROM sys_users));

-- 用户的登录凭证（密码统一为admin，哈希值与原admin一致，方便测试）
INSERT INTO public.sys_user_credentials (user_id, identity_type, identifier, credential_type, credential, status,
                                         is_primary, created_at)
VALUES (1, 'USERNAME', 'admin', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a',
        'ENABLED', true, now()),
       (1, 'EMAIL', 'admin@gmail.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a',
        'ENABLED', false, now())
;
SELECT setval('sys_user_credentials_id_seq', (SELECT MAX(id) FROM sys_user_credentials));

insert into sys_user_roles (id, tenant_id, user_id, role_id, start_at, end_at, assigned_at, assigned_by, is_primary, status, created_at)
VALUES (1, 0, 1, 1, null, null, now(), null, true, 'ACTIVE', now())
;

-- 权限分组
INSERT INTO sys_permission_groups (id, parent_id, path, name, module, sort_order, created_at)
VALUES (1, NULL, '/1/', '系统管理', 'sys', 1, now()),
       (2, 1, '/1/2/', '系统权限', 'sys', 1, now()),
       (3, 1, '/1/3/', '租户管理', 'sys', 2, now()),
       (4, 1, '/1/4/', '审计管理', 'sys', 3, now()),
       (5, 1, '/1/5/', '安全策略', 'sys', 4, now()),
;
SELECT setval('sys_permission_groups_id_seq', (SELECT MAX(id) FROM sys_permission_groups));

-- 权限点
INSERT INTO sys_permissions (id, group_id, name, code, description, status, created_at)
VALUES (1, 2, '访问后台', 'sys:access_backend', '允许用户访问系统后台管理界面', 'ON', now()),
       (2, 2, '平台管理员权限', 'sys:platform_admin', '拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源', 'ON', now()),
       (3, 3, '租户管理员权限', 'sys:tenant_manager', '拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源', 'ON', now()),
       (4, 3, '管理租户', 'sys:manage_tenants', '允许创建/修改/删除租户', 'ON', now()),
       (5, 4, '查看审计日志', 'sys:audit_logs', '允许查看系统操作日志', 'ON', now()),
;
SELECT setval('sys_permissions_id_seq', (SELECT MAX(id) FROM sys_permissions));

INSERT INTO public.sys_permission_apis (created_at, permission_id, api_id)
SELECT now(),
       2,
       unnest(ARRAY[1, 2, 3, 4, 5, 6, 7, 8, 9,
              10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
              20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
              30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
              40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
              50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
              60, 61, 62, 63, 64, 65, 66, 67, 68, 69,
              70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
              80, 81, 82, 83, 84, 85, 86, 87, 88, 89,
              90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
              100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
              110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
              120, 121, 122, 123, 124, 125, 126, 127, 128])
;
INSERT INTO public.sys_permission_menus (created_at, permission_id, menu_id)
SELECT now(),
       2,
       unnest(ARRAY[1, 2,
              10, 11,
              20, 21, 22, 23, 24,
              30, 31, 32, 33, 34,
              40, 41, 42,
              50, 51, 52,
              60, 61, 62, 63, 64])
;


INSERT INTO public.sys_permission_apis (created_at, permission_id, api_id)
SELECT now(),
       3,
       unnest(ARRAY[1, 2, 3, 4, 5, 6, 7, 8, 9,
              10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
              20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
              30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
              40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
              50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
              60, 61, 62, 63, 64, 65, 66, 67, 68, 69,
              70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
              80, 81, 82, 83, 84, 85, 86, 87, 88, 89,
              90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
              100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
              110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
              120, 121, 122, 123, 124, 125 ])
;
INSERT INTO public.sys_permission_menus (created_at, permission_id, menu_id)
SELECT now(),
       3,
       unnest(ARRAY[1, 2,
              20, 21, 22, 23, 24,
              30, 32,
              40, 41,
              50, 51,
              60, 61, 62, 63, 64,])
;

-- 默认的角色
INSERT INTO public.sys_roles(id, tenant_id, sort_order, name, code, status, is_protected, is_system, description, created_at)
VALUES (1, 0, 1, '平台管理员', 'platform:admin', 'ON', true, true, '拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源', now()),
       (2, 0, 2, '租户管理员模板', 'template:tenant:manager', 'ON', true, true, '租户管理员角色，拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源', now())
;
SELECT setval('sys_roles_id_seq', (SELECT MAX(id) FROM sys_roles));

INSERT INTO public.sys_role_metadata(id, tenant_id, role_id, is_template, scope, sync_policy, created_at)
VALUES (1, 0, 1, false, 'PLATFORM', 'AUTO', now()),
       (2, 0, 2, true, 'TENANT', 'AUTO', now())
;
SELECT setval('sys_role_metadata_id_seq', (SELECT MAX(id) FROM sys_role_metadata));

INSERT INTO public.sys_role_permissions (created_at, role_id, permission_id)
SELECT now(),
       1,
       unnest(ARRAY[1, 2, 4])
;
INSERT INTO public.sys_role_permissions (created_at, role_id, permission_id)
SELECT now(),
       2,
       unnest(ARRAY[1, 3])
;

-- 用户-租户关联关系
INSERT INTO public.sys_memberships (id, tenant_id, user_id, org_unit_id, position_id, role_id, is_primary, status)
VALUES
    -- 系统管理员（ADMIN）
    (1, 0, 1, null, null, 1, true, 'ACTIVE'),
;
SELECT setval('sys_memberships_id_seq', (SELECT MAX(id) FROM sys_memberships));

-- 租户成员-角色关联关系
INSERT INTO sys_membership_roles (id, membership_id, tenant_id, role_id, is_primary, status)
VALUES
    -- 系统管理员（ADMIN）;
    (1, 1, 0, 1, true, 'ACTIVE')
SELECT setval('sys_membership_roles_id_seq', (SELECT MAX(id) FROM sys_membership_roles));

-- 后台目录
INSERT INTO public.sys_menus(id, parent_id, type, name, path, redirect, component, status, created_at, meta)
VALUES (1, null, 'CATALOG', 'Dashboard', '/dashboard', null, 'BasicLayout', 'ON', now(),
        '{"order":-1, "title":"page.dashboard.title", "icon":"lucide:layout-dashboard", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (2, 1, 'MENU', 'Analytics', 'analytics', null, 'dashboard/analytics/index.vue', 'ON', now(),
        '{"order":-1, "title":"page.dashboard.analytics", "icon":"lucide:area-chart", "affixTab": true, "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (10, null, 'CATALOG', 'TenantManagement', '/tenant', null, 'BasicLayout', 'ON', now(),
        '{"order":2000, "title":"menu.tenant.moduleName", "icon":"lucide:building-2", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (11, 10, 'MENU', 'TenantMemberManagement', 'tenants', null, 'app/tenant/tenant/index.vue', 'ON', now(),
        '{"order":1, "title":"menu.tenant.member", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (20, null, 'CATALOG', 'OrganizationalPersonnelManagement', '/opm', null, 'BasicLayout', 'ON', now(),
        '{"order":2001, "title":"menu.opm.moduleName", "icon":"lucide:users", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (21, 20, 'MENU', 'OrgUnitManagement', 'org-units', null, 'app/opm/org_unit/index.vue', 'ON', now(),
        '{"order":1, "title":"menu.opm.orgUnit", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (22, 20, 'MENU', 'PositionManagement', 'positions', null, 'app/opm/position/index.vue', 'ON', now(),
        '{"order":3, "title":"menu.opm.position", "icon":"lucide:briefcase", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (23, 20, 'MENU', 'UserManagement', 'users', null, 'app/opm/users/index.vue', 'ON', now(),
        '{"order":4, "title":"menu.opm.user", "icon":"lucide:users", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (24, 20, 'MENU', 'UserDetail', 'users/detail/:id', null, 'app/opm/users/detail/index.vue', 'ON', now(),
        '{"order":1, "title":"menu.opm.userDetail", "icon":"", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":true, "hideInTab":false}'),

       (30, null, 'CATALOG', 'PermissionManagement', '/permission', null, 'BasicLayout', 'ON', now(),
        '{"order":2002, "title":"menu.permission.moduleName", "icon":"lucide:shield-check", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (31, 30, 'MENU', 'PermissionPointsManagement', 'permissions', null, 'app/permission/permission/index.vue', 'ON',
        now(),
        '{"order":1, "title":"menu.permission.permission", "icon":"lucide:shield-ellipsis", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (32, 30, 'MENU', 'RoleManagement', 'roles', null, 'app/permission/role/index.vue', 'ON', now(),
        '{"order":2, "title":"menu.permission.role", "icon":"lucide:shield-user", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (33, 30, 'MENU', 'MenuManagement', 'menus', null, 'app/permission/menu/index.vue', 'ON', now(),
        '{"order":3, "title":"menu.permission.menu", "icon":"lucide:square-menu", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (34, 30, 'MENU', 'APIManagement', 'apis', null, 'app/permission/api/index.vue', 'ON', now(),
        '{"order":4, "title":"menu.permission.api", "icon":"lucide:route", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (40, null, 'CATALOG', 'InternalMessageManagement', '/internal-message', null, 'BasicLayout', 'ON', now(),
        '{"order":2003, "title":"menu.internalMessage.moduleName", "icon":"lucide:mail", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (41, 40, 'MENU', 'InternalMessageList', 'messages', null, 'app/internal_message/message/index.vue', 'ON', now(),
        '{"order": 1, "title":"menu.internalMessage.internalMessage", "icon":"lucide:message-circle-more", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (42, 40, 'MENU', 'InternalMessageCategoryManagement', 'categories', null,
        'app/internal_message/category/index.vue', 'ON', now(),
        '{"order":2, "title":"menu.internalMessage.internalMessageCategory", "icon":"lucide:calendar-check", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (50, null, 'CATALOG', 'LogAuditManagement', '/log', null, 'BasicLayout', 'ON', now(),
        '{"order":2004, "title":"menu.log.moduleName", "icon":"lucide:activity", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (51, 50, 'MENU', 'LoginAuditLog', 'login-audit-logs', null, 'app/log/login_audit_log/index.vue', 'ON', now(),
        '{"order":1, "title":"menu.log.loginAuditLog", "icon":"lucide:user-lock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (52, 50, 'MENU', 'ApiAuditLog', 'api-audit-logs', null, 'app/log/api_audit_log/index.vue', 'ON', now(),
        '{"order":2, "title":"menu.log.apiAuditLog", "icon":"lucide:file-clock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (60, null, 'CATALOG', 'System', '/system', null, 'BasicLayout', 'ON', now(),
        '{"order":2005, "title":"menu.system.moduleName", "icon":"lucide:settings", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (61, 60, 'MENU', 'DictManagement', 'dict', null, 'app/system/dict/index.vue', 'ON', now(),
        '{"order":1, "title":"menu.system.dict", "icon":"lucide:library-big", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (62, 60, 'MENU', 'FileManagement', 'files', null, 'app/system/files/index.vue', 'ON', now(),
        '{"order":2, "title":"menu.system.file", "icon":"lucide:file-search", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (63, 60, 'MENU', 'TaskManagement', 'tasks', null, 'app/system/task/index.vue', 'ON', now(),
        '{"order":3, "title":"menu.system.task", "icon":"lucide:list-todo", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (64, 60, 'MENU', 'LoginPolicyManagement', 'login-policies', null,
        'app/system/login_policy/index.vue', 'ON', now(),
        '{"order":5, "title":"menu.system.loginPolicy", "icon":"lucide:shield-x", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}')
;
SELECT setval('sys_menus_id_seq', (SELECT MAX(id) FROM sys_menus));

COMMIT;
