-- Description: 初始化默认用户、角色、菜单和API资源数据
-- Note: 需要有表结构之后再执行此脚本。另，请确保在执行此脚本前已备份相关数据，以防数据丢失。

BEGIN;

SET LOCAL search_path = public, pg_catalog;

-- 一次性清理相关表并重置自增（包含外键依赖）
TRUNCATE TABLE public.sys_user_credentials,
               public.sys_users,
               public.sys_roles,
               public.sys_role_api,
               public.sys_role_menu,
               public.sys_tenants,
               public.sys_menus,
               public.sys_api_resources,
               public.sys_permissions,
               public.sys_memberships,
               public.sys_membership_org_units,
               public.sys_membership_positions,
               public.sys_membership_roles,
               public.sys_org_units
RESTART IDENTITY CASCADE;

-- 默认的角色
INSERT INTO public.sys_roles(id, parent_id, sort_order, name, code, type, data_scope, status, remark, created_at)
VALUES (1, null, 1, '超级管理员', 'super', 'SYSTEM', 'ALL', 'ON', '拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源', now()),
       (2, null, 2, '租户管理员', 'tenant_admin', 'SYSTEM', 'UNIT_AND_CHILD', 'ON', '管理当前租户下的用户、角色及资源，无跨租户操作权限', now()),
       (3, null, 3, '普通用户', 'user', 'SYSTEM', 'UNIT_AND_CHILD', 'ON', '可访问和使用租户内授权的资源，无管理权限', now()),
       (4, null, 4, '访客用户', 'guest', 'SYSTEM', 'UNIT_AND_CHILD', 'ON', '仅可访问公开资源，无修改和管理权限，会话过期后自动失效', now()),
       (5, null, 5, '审计员', 'auditor', 'SYSTEM', 'UNIT_AND_CHILD', 'ON', '仅可查看系统操作日志和数据记录，无修改权限', now())
;
SELECT setval('sys_roles_id_seq', (SELECT MAX(id) FROM sys_roles));

INSERT INTO public.sys_role_api (created_at, role_id, api_id)
SELECT now(),
       1,
       unnest(ARRAY [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114])
;
INSERT INTO public.sys_role_menu (created_at, role_id, menu_id)
SELECT now(),
       1,
       unnest(ARRAY [1, 2, 10, 11, 20, 21, 22, 23, 24, 25, 30, 31, 32, 40, 41, 42, 50, 51, 52, 60, 61, 62, 63, 64, 65])
;

INSERT INTO public.sys_role_api (created_at, role_id, api_id)
SELECT now(),
       2,
       unnest(ARRAY [105, 104, 35, 34, 16, 106, 93, 14, 1, 92, 91, 85, 79, 46, 24, 23, 78, 56, 55, 8, 7, 52, 51, 6, 5, 4, 31, 30, 20, 19, 53, 15])
;
INSERT INTO public.sys_role_menu (created_at, role_id, menu_id)
SELECT now(),
       2,
       unnest(ARRAY [1, 2, 20, 21, 22, 23, 24, 25, 50, 51, 52])
;

-- 租户
INSERT INTO public.sys_tenants(id, name, code, type, audit_status, status, admin_user_id, created_at)
VALUES
    (1, '测试租户', 'super', 'PAID', 'APPROVED', 'ON', 2, now())
;
SELECT setval('sys_tenants_id_seq', (SELECT MAX(id) FROM sys_tenants));

-- 插入4个权限的用户
INSERT INTO public.sys_users (id, tenant_id, username, nickname, realname, email, gender, created_at)
VALUES
    -- 1. 系统管理员（ADMIN）
    (1, 0, 'admin', '鹳狸猿', '喵个咪', 'admin@gmail.com', 'MALE', now()),
   -- 2. 租户管理员（TENANT_ADMIN）
   (2, 1, 'tenant_admin', '租户管理', '张管理员', 'tenant@company.com', 'MALE', now()),
   -- 3. 普通用户（USER）
   (3, 0, 'normal_user', '普通用户', '李用户', 'user@company.com', 'FEMALE', now()),
   -- 4. 访客（GUEST）
   (4, 0, 'guest_user', '临时访客', '王访客', 'guest@company.com', 'SECRET', now())
;
SELECT setval('sys_users_id_seq', (SELECT MAX(id) FROM sys_users));

-- 插入4个用户的凭证（密码统一为admin，哈希值与原admin一致，方便测试）
INSERT INTO public.sys_user_credentials (user_id, identity_type, identifier, credential_type, credential, status, is_primary, created_at)
VALUES (1, 'USERNAME', 'admin', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', true, now()),
       (1, 'EMAIL', 'admin@gmail.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', false, now()),
       -- 租户管理员（对应users表id=2）
       (2, 'USERNAME', 'tenant_admin', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', true, now()),
       (2, 'EMAIL', 'tenant@company.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', false, now()),

       -- 普通用户（对应users表id=3）
       (3, 'USERNAME', 'normal_user', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', true, now()),
       (3, 'EMAIL', 'user@company.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', false, now()),

       -- 访客（对应users表id=4）
       (4, 'USERNAME', 'guest_user', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', true, now()),
       (4, 'EMAIL', 'guest@company.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a', 'ENABLED', false, now())
;
SELECT setval('sys_user_credentials_id_seq', (SELECT MAX(id) FROM sys_user_credentials));

-- 用户-租户关联关系
INSERT INTO public.sys_memberships (id, tenant_id, user_id, org_unit_id, position_id, role_id, is_primary, status)
VALUES
         -- 系统管理员（ADMIN）
         (1, 0, 1, null, null, 1, true, 'ACTIVE'),
         -- 租户管理员（TENANT_ADMIN）
         (2, 1, 2, null, null, 2, true, 'ACTIVE'),
         -- 普通用户（USER）
         (3, 0, 3, null, null, 3, true, 'ACTIVE'),
         -- 访客（GUEST）
         (4, 0, 4, null, null, 4, true, 'ACTIVE')
;
SELECT setval('sys_memberships_id_seq', (SELECT MAX(id) FROM sys_memberships));

-- 用户-角色关联关系
INSERT INTO sys_membership_roles (id, membership_id, tenant_id, role_id, is_primary, status)
VALUES
    -- 系统管理员（ADMIN）;
    (1, 1, 0, 1, true, 'ACTIVE'),
    -- 租户管理员（TENANT_ADMIN）
    (2, 2, 1, 2, true, 'ACTIVE'),
    -- 普通用户（USER）
    (3, 3, 0, 3, true, 'ACTIVE'),
    -- 访客（GUEST）
    (4, 4, 0, 4, true, 'ACTIVE')
SELECT setval('sys_membership_roles_id_seq', (SELECT MAX(id) FROM sys_membership_roles));

-- 后台目录
INSERT INTO public.sys_menus(id, parent_id, type, name, path, redirect, component, status, created_at, meta)
VALUES (1, null, 'FOLDER', 'Dashboard', '/', null, 'BasicLayout', 'ON', now(), '{"order":-1, "title":"page.dashboard.title", "icon":"lucide:layout-dashboard", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (2, 1, 'MENU', 'Analytics', '/analytics', null, 'dashboard/analytics/index.vue', 'ON', now(), '{"order":-1, "title":"page.dashboard.analytics", "icon":"lucide:area-chart", "affixTab": true, "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (10, null, 'FOLDER', 'TenantManagement', '/tenant', null, 'BasicLayout', 'ON', now(), '{"order":2000, "title":"menu.tenant.moduleName", "icon":"lucide:building-2", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (11, 10, 'MENU', 'TenantMemberManagement', 'members', null, 'app/tenant/tenant/index.vue', 'ON', now(), '{"order":1, "title":"menu.tenant.member", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (20, null, 'FOLDER', 'OrganizationalPersonnelManagement', '/opm', null, 'BasicLayout', 'ON', now(), '{"order":2001, "title":"menu.opm.moduleName", "icon":"lucide:users", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (21, 20, 'MENU', 'OrgUnitManagement', 'org-units', null, 'app/opm/org_unit/index.vue', 'ON', now(), '{"order":1, "title":"menu.opm.orgUnit", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (22, 20, 'MENU', 'PositionManagement', 'positions', null, 'app/opm/position/index.vue', 'ON', now(), '{"order":3, "title":"menu.opm.position", "icon":"lucide:briefcase", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (23, 20, 'MENU', 'UserManagement', 'users', null, 'app/opm/users/index.vue', 'ON', now(), '{"order":4, "title":"menu.opm.user", "icon":"lucide:users", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (24, 20, 'MENU', 'UserDetail', 'users/detail/:id', null, 'app/opm/users/detail/index.vue', 'ON', now(), '{"order":1, "title":"menu.opm.userDetail", "icon":"", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":true, "hideInTab":false}'),

       (30, null, 'FOLDER', 'PermissionManagement', '/permission', null, 'BasicLayout', 'ON', now(), '{"order":2002, "title":"menu.permission.moduleName", "icon":"lucide:shield-check", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (31, 30, 'MENU', 'RoleManagement', 'roles', null, 'app/permission/role/index.vue', 'ON', now(), '{"order":1, "title":"menu.permission.role", "icon":"lucide:shield-user", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (32, 30, 'MENU', 'MenuManagement', 'menus', null, 'app/permission/menu/index.vue', 'ON', now(), '{"order":2, "title":"menu.permission.menu", "icon":"lucide:square-menu", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (40, null, 'FOLDER', 'InternalMessageManagement', '/internal_message', null, 'BasicLayout', 'ON', now(), '{"order":2003, "title":"menu.internalMessage.moduleName", "icon":"lucide:mail", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (41, 40, 'MENU', 'InternalMessageList', 'messages', null, 'app/internal_message/message/index.vue', 'ON', now(), '{"order": 1, "title":"menu.internalMessage.internalMessage", "icon":"lucide:message-circle-more", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (42, 40, 'MENU', 'InternalMessageCategoryManagement', 'categories', null, 'app/internal_message/category/index.vue', 'ON', now(), '{"order":2, "title":"menu.internalMessage.internalMessageCategory", "icon":"lucide:calendar-check", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (50, null, 'FOLDER', 'LogAuditManagement', '/log', null, 'BasicLayout', 'ON', now(), '{"order":2004, "title":"menu.log.moduleName", "icon":"lucide:activity", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (51, 50, 'MENU', 'AdminLoginLog', 'login', null, 'app/log/admin_login_log/index.vue', 'ON', now(), '{"order":1, "title":"menu.log.adminLoginLog", "icon":"lucide:user-lock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (52, 50, 'MENU', 'AdminOperationLog', 'operation', null, 'app/log/admin_operation_log/index.vue', 'ON', now(), '{"order":2, "title":"menu.log.adminOperationLog", "icon":"lucide:file-clock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (60, null, 'FOLDER', 'System', '/system', null, 'BasicLayout', 'ON', now(), '{"order":2005, "title":"menu.system.moduleName", "icon":"lucide:settings", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (61, 60, 'MENU', 'DictManagement', 'dict', null, 'app/system/dict/index.vue', 'ON', now(), '{"order":1, "title":"menu.system.dict", "icon":"lucide:library-big", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (62, 60, 'MENU', 'FileManagement', 'files', null, 'app/system/files/index.vue', 'ON', now(), '{"order":2, "title":"menu.system.file", "icon":"lucide:file-search", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (63, 60, 'MENU', 'TaskManagement', 'tasks', null, 'app/system/task/index.vue', 'ON', now(), '{"order":3, "title":"menu.system.task", "icon":"lucide:list-todo", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (64, 60, 'MENU', 'APIResourceManagement', 'apis', null, 'app/system/api_resource/index.vue', 'ON', now(), '{"order":4, "title":"menu.system.apiResource", "icon":"lucide:route", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (65, 60, 'MENU', 'AdminLoginRestrictionManagement', 'admin_login_restriction', null, 'app/system/admin_login_restriction/index.vue', 'ON', now(), '{"order":5, "title":"menu.system.adminLoginRestriction", "icon":"lucide:shield-x", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}')
;
SELECT setval('sys_menus_id_seq', (SELECT MAX(id) FROM sys_menus));

-- API资源表数据
INSERT INTO public.sys_api_resources (
    id,
    created_at,
    description,
    module,
    module_description,
    operation,
    path,
    method,
    scope
) VALUES
      (1, now(), '分页查询字典类型列表', 'DictService', '数据字典管理服务', 'DictService_ListDictType', '/admin/v1/dict-types', 'GET', 'ADMIN'),
      (2, now(), '创建字典类型', 'DictService', '数据字典管理服务', 'DictService_CreateDictType', '/admin/v1/dict-types', 'POST', 'ADMIN'),
      (3, now(), '删除字典类型', 'DictService', '数据字典管理服务', 'DictService_DeleteDictType', '/admin/v1/dict-types', 'DELETE', 'ADMIN'),
      (4, now(), '发送消息', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_SendMessage', '/admin/v1/internal-message/send', 'POST', 'ADMIN'),
      (5, now(), '修改用户密码', 'UserProfileService', '用户个人资料服务', 'UserProfileService_ChangePassword', '/admin/v1/me/password', 'POST', 'ADMIN'),
      (6, now(), '查询权限码列表', 'RouterService', '网站后台动态路由服务', 'RouterService_ListPermissionCode', '/admin/v1/perm-codes', 'GET', 'ADMIN'),
      (7, now(), '更新字典条目', 'DictService', '数据字典管理服务', 'DictService_UpdateDictEntry', '/admin/v1/dict-entries/{id}', 'PUT', 'ADMIN'),
      (8, now(), '刷新认证令牌', 'AuthenticationService', '用户后台登录认证服务', 'AuthenticationService_RefreshToken', '/admin/v1/refresh_token', 'POST', 'ADMIN'),
      (9, now(), '同步API资源', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_SyncApiResources', '/admin/v1/api-resources/sync', 'POST', 'ADMIN'),
      (10, now(), '删除站内信消息', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_DeleteMessage', '/admin/v1/internal-message/messages/{id}', 'DELETE', 'ADMIN'),
      (11, now(), '查询站内信消息详情', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_GetMessage', '/admin/v1/internal-message/messages/{id}', 'GET', 'ADMIN'),
      (12, now(), '更新站内信消息', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_UpdateMessage', '/admin/v1/internal-message/messages/{id}', 'PUT', 'ADMIN'),
      (13, now(), '查询后台操作日志详情', 'AdminOperationLogService', '后台操作日志管理服务', 'AdminOperationLogService_Get', '/admin/v1/admin_operation_logs/{id}', 'GET', 'ADMIN'),
      (14, now(), '查询后台登录日志详情', 'AdminLoginLogService', '后台登录日志管理服务', 'AdminLoginLogService_Get', '/admin/v1/admin_login_logs/{id}', 'GET', 'ADMIN'),
      (15, now(), '修改用户密码', 'UserService', '用户管理服务', 'UserService_EditUserPassword', '/admin/v1/users/{userId}/password', 'POST', 'ADMIN'),
      (16, now(), '控制调度任务', 'TaskService', '调度任务管理服务', 'TaskService_ControlTask', '/admin/v1/tasks:control', 'POST', 'ADMIN'),
      (17, now(), '删除用户', 'UserService', '用户管理服务', 'UserService_Delete', '/admin/v1/users/{id}', 'DELETE', 'ADMIN'),
      (18, now(), '获取用户数据', 'UserService', '用户管理服务', 'UserService_Get', '/admin/v1/users/{id}', 'GET', 'ADMIN'),
      (19, now(), '更新用户', 'UserService', '用户管理服务', 'UserService_Update', '/admin/v1/users/{id}', 'PUT', 'ADMIN'),
      (20, now(), '查询文件列表', 'FileService', '文件管理服务', 'FileService_List', '/admin/v1/files', 'GET', 'ADMIN'),
      (21, now(), '创建文件', 'FileService', '文件管理服务', 'FileService_Create', '/admin/v1/files', 'POST', 'ADMIN'),
      (22, now(), '更新菜单', 'MenuService', '后台菜单管理服务', 'MenuService_Update', '/admin/v1/menus/{id}', 'PUT', 'ADMIN'),
      (23, now(), '删除菜单', 'MenuService', '后台菜单管理服务', 'MenuService_Delete', '/admin/v1/menus/{id}', 'DELETE', 'ADMIN'),
      (24, now(), '查询菜单详情', 'MenuService', '后台菜单管理服务', 'MenuService_Get', '/admin/v1/menus/{id}', 'GET', 'ADMIN'),
      (25, now(), '上传文件', 'UEditorService', 'UEditor后端服务', 'UEditorService_UploadFile', '/admin/v1/ueditor', 'POST', 'ADMIN'),
      (26, now(), 'UEditor API', 'UEditorService', 'UEditor后端服务', 'UEditorService_UEditorAPI', '/admin/v1/ueditor', 'GET', 'ADMIN'),
      (27, now(), '删除后台登录限制', 'AdminLoginRestrictionService', '后台登录限制管理服务', 'AdminLoginRestrictionService_Delete', '/admin/v1/login-restrictions/{id}', 'DELETE', 'ADMIN'),
      (28, now(), '查询后台登录限制详情', 'AdminLoginRestrictionService', '后台登录限制管理服务', 'AdminLoginRestrictionService_Get', '/admin/v1/login-restrictions/{id}', 'GET', 'ADMIN'),
      (29, now(), '更新后台登录限制', 'AdminLoginRestrictionService', '后台登录限制管理服务', 'AdminLoginRestrictionService_Update', '/admin/v1/login-restrictions/{id}', 'PUT', 'ADMIN'),
      (30, now(), '上传头像', 'UserProfileService', '用户个人资料服务', 'UserProfileService_UploadAvatar', '/admin/v1/me/avatar', 'POST', 'ADMIN'),
      (31, now(), '删除头像', 'UserProfileService', '用户个人资料服务', 'UserProfileService_DeleteAvatar', '/admin/v1/me/avatar', 'DELETE', 'ADMIN'),
      (32, now(), '获取用户数据', 'UserService', '用户管理服务', 'UserService_Get', '/admin/v1/users/username/{username}', 'GET', 'ADMIN'),
      (33, now(), '删除用户', 'UserService', '用户管理服务', 'UserService_Delete', '/admin/v1/users/username/{username}', 'DELETE', 'ADMIN'),
      (34, now(), '查询后台登录日志列表', 'AdminLoginLogService', '后台登录日志管理服务', 'AdminLoginLogService_List', '/admin/v1/admin_login_logs', 'GET', 'ADMIN'),
      (35, now(), '查询职位列表', 'PositionService', '职位管理服务', 'PositionService_List', '/admin/v1/positions', 'GET', 'ADMIN'),
      (36, now(), '创建职位', 'PositionService', '职位管理服务', 'PositionService_Create', '/admin/v1/positions', 'POST', 'ADMIN'),
      (37, now(), '删除租户', 'TenantService', '租户管理服务', 'TenantService_Delete', '/admin/v1/tenants/{id}', 'DELETE', 'ADMIN'),
      (38, now(), '获取租户数据', 'TenantService', '租户管理服务', 'TenantService_Get', '/admin/v1/tenants/{id}', 'GET', 'ADMIN'),
      (39, now(), '更新租户', 'TenantService', '租户管理服务', 'TenantService_Update', '/admin/v1/tenants/{id}', 'PUT', 'ADMIN'),
      (40, now(), '获取用户资料', 'UserProfileService', '用户个人资料服务', 'UserProfileService_GetUser', '/admin/v1/me', 'GET', 'ADMIN'),
      (41, now(), '更新用户资料', 'UserProfileService', '用户个人资料服务', 'UserProfileService_UpdateUser', '/admin/v1/me', 'PUT', 'ADMIN'),
      (42, now(), '创建租户及管理员用户', 'TenantService', '租户管理服务', 'TenantService_CreateTenantWithAdminUser', '/admin/v1/tenants_with_admin', 'POST', 'ADMIN'),
      (43, now(), '获取用户的收件箱列表 (通知类)', 'InternalMessageRecipientService', '站内信消息管理服务', 'InternalMessageRecipientService_ListUserInbox', '/admin/v1/internal-message/inbox', 'GET', 'ADMIN'),
      (44, now(), '查询后台登录限制列表', 'AdminLoginRestrictionService', '后台登录限制管理服务', 'AdminLoginRestrictionService_List', '/admin/v1/login-restrictions', 'GET', 'ADMIN'),
      (45, now(), '创建后台登录限制', 'AdminLoginRestrictionService', '后台登录限制管理服务', 'AdminLoginRestrictionService_Create', '/admin/v1/login-restrictions', 'POST', 'ADMIN'),
      (46, now(), '查询调度任务详情', 'TaskService', '调度任务管理服务', 'TaskService_Get', '/admin/v1/tasks/type-name/{typeName}', 'GET', 'ADMIN'),
      (47, now(), '查询调度任务详情', 'TaskService', '调度任务管理服务', 'TaskService_Get', '/admin/v1/tasks/{id}', 'GET', 'ADMIN'),
      (48, now(), '更新调度任务', 'TaskService', '调度任务管理服务', 'TaskService_Update', '/admin/v1/tasks/{id}', 'PUT', 'ADMIN'),
      (49, now(), '删除调度任务', 'TaskService', '调度任务管理服务', 'TaskService_Delete', '/admin/v1/tasks/{id}', 'DELETE', 'ADMIN'),
      (50, now(), '停止所有的调度任务', 'TaskService', '调度任务管理服务', 'TaskService_StopAllTask', '/admin/v1/tasks:stop', 'POST', 'ADMIN'),
      (51, now(), '更新API资源', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_Update', '/admin/v1/api-resources/{id}', 'PUT', 'ADMIN'),
      (52, now(), '删除API资源', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_Delete', '/admin/v1/api-resources/{id}', 'DELETE', 'ADMIN'),
      (53, now(), '查询API资源详情', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_Get', '/admin/v1/api-resources/{id}', 'GET', 'ADMIN'),
      (54, now(), '验证手机号码/邮箱', 'UserProfileService', '用户个人资料服务', 'UserProfileService_VerifyContact', '/admin/v1/me/contact/verify', 'POST', 'ADMIN'),
      (55, now(), '将通知标记为已读', 'InternalMessageRecipientService', '站内信消息管理服务', 'InternalMessageRecipientService_MarkNotificationAsRead', '/admin/v1/internal-message/read', 'POST', 'ADMIN'),
      (56, now(), '查询权限列表', 'PermissionService', '权限管理服务', 'PermissionService_List', '/admin/v1/permissions', 'GET', 'ADMIN'),
      (57, now(), '创建权限', 'PermissionService', '权限管理服务', 'PermissionService_Create', '/admin/v1/permissions', 'POST', 'ADMIN'),
      (58, now(), '查询路由列表', 'RouterService', '网站后台动态路由服务', 'RouterService_ListRoute', '/admin/v1/routes', 'GET', 'ADMIN'),
      (59, now(), '用户是否存在', 'UserService', '用户管理服务', 'UserService_UserExists', '/admin/v1/users:exists', 'GET', 'ADMIN'),
      (60, now(), '启动所有的调度任务', 'TaskService', '调度任务管理服务', 'TaskService_StartAllTask', '/admin/v1/tasks:start', 'POST', 'ADMIN'),
      (61, now(), '分页查询字典条目列表', 'DictService', '数据字典管理服务', 'DictService_ListDictEntry', '/admin/v1/dict-entries', 'GET', 'ADMIN'),
      (62, now(), '创建字典条目', 'DictService', '数据字典管理服务', 'DictService_CreateDictEntry', '/admin/v1/dict-entries', 'POST', 'ADMIN'),
      (63, now(), '删除字典条目', 'DictService', '数据字典管理服务', 'DictService_DeleteDictEntry', '/admin/v1/dict-entries', 'DELETE', 'ADMIN'),
      (64, now(), '查询组织单元列表', 'OrgUnitService', '组织单元服务', 'OrgUnitService_List', '/admin/v1/org_units', 'GET', 'ADMIN'),
      (65, now(), '创建组织单元', 'OrgUnitService', '组织单元服务', 'OrgUnitService_Create', '/admin/v1/org_units', 'POST', 'ADMIN'),
      (66, now(), '获取对象存储（OSS）上传用的预签名链接', 'OssService', 'OSS服务', 'OssService_OssUploadUrl', '/admin/v1/file:upload-url', 'POST', 'ADMIN'),
      (67, now(), '登录', 'AuthenticationService', '用户后台登录认证服务', 'AuthenticationService_Login', '/admin/v1/login', 'POST', 'ADMIN'),
      (68, now(), '登出', 'AuthenticationService', '用户后台登录认证服务', 'AuthenticationService_Logout', '/admin/v1/logout', 'POST', 'ADMIN'),
      (69, now(), '删除角色', 'RoleService', '角色管理服务', 'RoleService_Delete', '/admin/v1/roles/{id}', 'DELETE', 'ADMIN'),
      (70, now(), '查询角色详情', 'RoleService', '角色管理服务', 'RoleService_Get', '/admin/v1/roles/{id}', 'GET', 'ADMIN'),
      (71, now(), '更新角色', 'RoleService', '角色管理服务', 'RoleService_Update', '/admin/v1/roles/{id}', 'PUT', 'ADMIN'),
      (72, now(), 'POST方法上传文件', 'OssService', 'OSS服务', 'OssService_PostUploadFile', '/admin/v1/file:upload', 'POST', 'ADMIN'),
      (73, now(), 'PUT方法上传文件', 'OssService', 'OSS服务', 'OssService_PutUploadFile', '/admin/v1/file:upload', 'PUT', 'ADMIN'),
      (74, now(), '绑定手机号码/邮箱', 'UserProfileService', '用户个人资料服务', 'UserProfileService_BindContact', '/admin/v1/me/contact', 'POST', 'ADMIN'),
      (75, now(), '删除文件', 'FileService', '文件管理服务', 'FileService_Delete', '/admin/v1/files/{id}', 'DELETE', 'ADMIN'),
      (76, now(), '查询文件详情', 'FileService', '文件管理服务', 'FileService_Get', '/admin/v1/files/{id}', 'GET', 'ADMIN'),
      (77, now(), '更新文件', 'FileService', '文件管理服务', 'FileService_Update', '/admin/v1/files/{id}', 'PUT', 'ADMIN'),
      (78, now(), '撤销某条消息', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_RevokeMessage', '/admin/v1/internal-message/revoke', 'POST', 'ADMIN'),
      (79, now(), '删除站内信消息分类', 'InternalMessageCategoryService', '站内信消息分类管理服务', 'InternalMessageCategoryService_Delete', '/admin/v1/internal-message/categories/{id}', 'DELETE', 'ADMIN'),
      (80, now(), '查询站内信消息分类详情', 'InternalMessageCategoryService', '站内信消息分类管理服务', 'InternalMessageCategoryService_Get', '/admin/v1/internal-message/categories/{id}', 'GET', 'ADMIN'),
      (81, now(), '更新站内信消息分类', 'InternalMessageCategoryService', '站内信消息分类管理服务', 'InternalMessageCategoryService_Update', '/admin/v1/internal-message/categories/{id}', 'PUT', 'ADMIN'),
      (82, now(), '查询字典类型详情', 'DictService', '数据字典管理服务', 'DictService_GetDictType', '/admin/v1/dict-types/{id}', 'GET', 'ADMIN'),
      (83, now(), '更新字典类型', 'DictService', '数据字典管理服务', 'DictService_UpdateDictType', '/admin/v1/dict-types/{id}', 'PUT', 'ADMIN'),
      (84, now(), '获取用户列表', 'UserService', '用户管理服务', 'UserService_List', '/admin/v1/users', 'GET', 'ADMIN'),
      (85, now(), '创建用户', 'UserService', '用户管理服务', 'UserService_Create', '/admin/v1/users', 'POST', 'ADMIN'),
      (86, now(), '更新组织单元', 'OrgUnitService', '组织单元服务', 'OrgUnitService_Update', '/admin/v1/org_units/{id}', 'PUT', 'ADMIN'),
      (87, now(), '删除组织单元', 'OrgUnitService', '组织单元服务', 'OrgUnitService_Delete', '/admin/v1/org_units/{id}', 'DELETE', 'ADMIN'),
      (88, now(), '查询组织单元详情', 'OrgUnitService', '组织单元服务', 'OrgUnitService_Get', '/admin/v1/org_units/{id}', 'GET', 'ADMIN'),
      (89, now(), '查询字典类型详情', 'DictService', '数据字典管理服务', 'DictService_GetDictType', '/admin/v1/dict-types/code/{code}', 'GET', 'ADMIN'),
      (90, now(), '查询站内信消息分类列表', 'InternalMessageCategoryService', '站内信消息分类管理服务', 'InternalMessageCategoryService_List', '/admin/v1/internal-message/categories', 'GET', 'ADMIN'),
      (91, now(), '创建站内信消息分类', 'InternalMessageCategoryService', '站内信消息分类管理服务', 'InternalMessageCategoryService_Create', '/admin/v1/internal-message/categories', 'POST', 'ADMIN'),
      (92, now(), '删除用户收件箱中的通知记录', 'InternalMessageRecipientService', '站内信消息管理服务', 'InternalMessageRecipientService_DeleteNotificationFromInbox', '/admin/v1/internal-message/inbox/delete', 'POST', 'ADMIN'),
      (93, now(), '更新职位', 'PositionService', '职位管理服务', 'PositionService_Update', '/admin/v1/positions/{id}', 'PUT', 'ADMIN'),
      (94, now(), '删除职位', 'PositionService', '职位管理服务', 'PositionService_Delete', '/admin/v1/positions/{id}', 'DELETE', 'ADMIN'),
      (95, now(), '查询职位详情', 'PositionService', '职位管理服务', 'PositionService_Get', '/admin/v1/positions/{id}', 'GET', 'ADMIN'),
      (96, now(), '创建API资源', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_Create', '/admin/v1/api-resources', 'POST', 'ADMIN'),
      (97, now(), '查询API资源列表', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_List', '/admin/v1/api-resources', 'GET', 'ADMIN'),
      (98, now(), '查询调度任务列表', 'TaskService', '调度任务管理服务', 'TaskService_List', '/admin/v1/tasks', 'GET', 'ADMIN'),
      (99, now(), '创建调度任务', 'TaskService', '调度任务管理服务', 'TaskService_Create', '/admin/v1/tasks', 'POST', 'ADMIN'),
      (100, now(), '租户是否存在', 'TenantService', '租户管理服务', 'TenantService_TenantExists', '/admin/v1/tenants_exists', 'GET', 'ADMIN'),
      (101, now(), '查询站内信消息列表', 'InternalMessageService', '站内信消息管理服务', 'InternalMessageService_ListMessage', '/admin/v1/internal-message/messages', 'GET', 'ADMIN'),
      (102, now(), '查询后台操作日志列表', 'AdminOperationLogService', '后台操作日志管理服务', 'AdminOperationLogService_List', '/admin/v1/admin_operation_logs', 'GET', 'ADMIN'),
      (103, now(), '任务类型名称列表', 'TaskService', '调度任务管理服务', 'TaskService_ListTaskTypeName', '/admin/v1/tasks:type-names', 'GET', 'ADMIN'),
      (104, now(), '查询角色列表', 'RoleService', '角色管理服务', 'RoleService_List', '/admin/v1/roles', 'GET', 'ADMIN'),
      (105, now(), '创建角色', 'RoleService', '角色管理服务', 'RoleService_Create', '/admin/v1/roles', 'POST', 'ADMIN'),
      (106, now(), '创建菜单', 'MenuService', '后台菜单管理服务', 'MenuService_Create', '/admin/v1/menus', 'POST', 'ADMIN'),
      (107, now(), '查询菜单列表', 'MenuService', '后台菜单管理服务', 'MenuService_List', '/admin/v1/menus', 'GET', 'ADMIN'),
      (108, now(), '重启所有的调度任务', 'TaskService', '调度任务管理服务', 'TaskService_RestartAllTask', '/admin/v1/tasks:restart', 'POST', 'ADMIN'),
      (109, now(), '获取租户列表', 'TenantService', '租户管理服务', 'TenantService_List', '/admin/v1/tenants', 'GET', 'ADMIN'),
      (110, now(), '创建租户', 'TenantService', '租户管理服务', 'TenantService_Create', '/admin/v1/tenants', 'POST', 'ADMIN'),
      (111, now(), '查询路由数据', 'ApiResourceService', 'API资源管理服务', 'ApiResourceService_GetWalkRouteData', '/admin/v1/api-resources/walk-route', 'GET', 'ADMIN'),
      (112, now(), '查询权限详情', 'PermissionService', '权限管理服务', 'PermissionService_Get', '/admin/v1/permissions/{id}', 'GET', 'ADMIN'),
      (113, now(), '更新权限', 'PermissionService', '权限管理服务', 'PermissionService_Update', '/admin/v1/permissions/{id}', 'PUT', 'ADMIN'),
      (114, now(), '删除权限', 'PermissionService', '权限管理服务', 'PermissionService_Delete', '/admin/v1/permissions/{id}', 'DELETE', 'ADMIN')
;
SELECT setval('sys_api_resources_id_seq', (SELECT MAX(id) FROM sys_api_resources));

COMMIT;
