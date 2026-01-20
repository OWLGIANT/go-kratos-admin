-- Description: 初始化默认用户、角色、菜单和API资源数据(MYSQL版)
-- Note: 需要有表结构之后再执行此脚本；执行前备份数据，MySQL需支持JSON字段（5.7+）
DELIMITER // -- 临时修改语句结束符，适配存储过程
SET FOREIGN_KEY_CHECKS = 0; -- 关闭外键检查，允许TRUNCATE关联表
START TRANSACTION; -- 开启事务，保证数据原子性

-- 一次性清理相关表（修复原脚本重复truncate sys_permissions的错误）
TRUNCATE TABLE sys_user_credentials AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_users AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_user_org_units AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_user_positions AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_user_roles AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_tenants AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_org_units AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_positions AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_roles AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_role_permissions AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_menus AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_apis AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_permissions AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_permission_groups AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_permission_apis AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_permission_menus AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_permission_policies AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_memberships AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_membership_org_units AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_membership_positions AUTO_INCREMENT = 1;
TRUNCATE TABLE sys_membership_roles AUTO_INCREMENT = 1;

-- ==============================================
-- 1. 插入默认用户
-- ==============================================
INSERT INTO sys_users (id, tenant_id, username, nickname, realname, email, gender, created_at)
VALUES
    -- 1. 系统管理员（ADMIN）
    (1, 0, 'admin', '鹳狸猿', '喵个咪', 'admin@gmail.com', 'MALE', NOW());
-- 重置自增（后续新增从MAX(id)+1开始）
ALTER TABLE sys_users AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_users);

-- ==============================================
-- 2. 插入用户登录凭证（密码：admin）
-- ==============================================
INSERT INTO sys_user_credentials (user_id, identity_type, identifier, credential_type, credential, status,
                                  is_primary, created_at)
VALUES (1, 'USERNAME', 'admin', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a',
        'ENABLED', true, NOW()),
       (1, 'EMAIL', 'admin@gmail.com', 'PASSWORD_HASH', '$2a$10$yajZDX20Y40FkG0Bu4N19eXNqRizez/S9fK63.JxGkfLq.RoNKR/a',
        'ENABLED', false, NOW());
ALTER TABLE sys_user_credentials AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_user_credentials);

-- ==============================================
-- 3. 插入用户-角色关联
-- ==============================================
INSERT INTO sys_user_roles (id, tenant_id, user_id, role_id, start_at, end_at, assigned_at, assigned_by, is_primary, status, created_at)
VALUES (1, 0, 1, 1, null, null, NOW(), null, true, 'ACTIVE', NOW());
ALTER TABLE sys_user_roles AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_user_roles);

-- ==============================================
-- 4. 插入权限分组
-- ==============================================
INSERT INTO sys_permission_groups (id, parent_id, path, name, module, sort_order, created_at)
VALUES (1, NULL, '/1/', '系统管理', 'sys', 1, NOW()),
       (2, 1, '/1/2/', '系统权限', 'sys', 1, NOW()),
       (3, 1, '/1/3/', '租户管理', 'sys', 2, NOW()),
       (4, 1, '/1/4/', '审计管理', 'sys', 3, NOW()),
       (5, 1, '/1/5/', '安全策略', 'sys', 4, NOW());
ALTER TABLE sys_permission_groups AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_permission_groups);

-- ==============================================
-- 5. 插入权限点
-- ==============================================
INSERT INTO sys_permissions (id, group_id, name, code, description, status, created_at)
VALUES (1, 2, '访问后台', 'sys:access_backend', '允许用户访问系统后台管理界面', 'ON', NOW()),
       (2, 2, '平台管理员权限', 'sys:platform_admin', '拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源', 'ON', NOW()),
       (3, 3, '租户管理员权限', 'sys:tenant_manager', '拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源', 'ON', NOW()),
       (4, 3, '管理租户', 'sys:manage_tenants', '允许创建/修改/删除租户', 'ON', NOW()),
       (5, 4, '查看审计日志', 'sys:audit_logs', '允许查看系统操作日志', 'ON', NOW());
ALTER TABLE sys_permissions AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_permissions);

-- ==============================================
-- 6. 批量插入【平台管理员权限-API】关联（替代PG的unnest，生成api_id:1-128）
-- ==============================================
CREATE PROCEDURE batch_insert_perm_api1()
BEGIN
    DECLARE i INT DEFAULT 1;
    WHILE i <= 128 DO
        INSERT INTO sys_permission_apis (created_at, permission_id, api_id) VALUES (NOW(), 2, i);
        SET i = i + 1;
END WHILE;
END //
CALL batch_insert_perm_api1() //
DROP PROCEDURE IF EXISTS batch_insert_perm_api1 //

-- ==============================================
-- 7. 批量插入【平台管理员权限-菜单】关联（替代PG的unnest，menu_id数组）
-- ==============================================
CREATE PROCEDURE batch_insert_perm_menu1()
BEGIN
    -- 定义menu_id数组
    SET @menu_ids = '1,2,10,11,20,21,22,23,24,30,31,32,33,34,40,41,42,50,51,52,60,61,62,63,64';
    SET @i = 1;
    SET @len = LENGTH(@menu_ids) - LENGTH(REPLACE(@menu_ids, ',', '')) + 1;
    WHILE @i <= @len DO
        SET @menu_id = SUBSTRING_INDEX(SUBSTRING_INDEX(@menu_ids, ',', @i), ',', -1);
INSERT INTO sys_permission_menus (created_at, permission_id, menu_id) VALUES (NOW(), 2, @menu_id);
SET @i = @i + 1;
END WHILE;
END //
CALL batch_insert_perm_menu1() //
DROP PROCEDURE IF EXISTS batch_insert_perm_menu1 //

-- ==============================================
-- 8. 批量插入【租户管理员权限-API】关联（替代PG的unnest，生成api_id:1-125）
-- ==============================================
CREATE PROCEDURE batch_insert_perm_api2()
BEGIN
    DECLARE i INT DEFAULT 1;
    WHILE i <= 125 DO
        INSERT INTO sys_permission_apis (created_at, permission_id, api_id) VALUES (NOW(), 3, i);
        SET i = i + 1;
END WHILE;
END //
CALL batch_insert_perm_api2() //
DROP PROCEDURE IF EXISTS batch_insert_perm_api2 //

-- ==============================================
-- 9. 批量插入【租户管理员权限-菜单】关联（替代PG的unnest，menu_id数组）
-- ==============================================
CREATE PROCEDURE batch_insert_perm_menu2()
BEGIN
    -- 定义menu_id数组（修复原脚本末尾多余逗号）
    SET @menu_ids = '1,2,20,21,22,23,24,30,32,40,41,50,51,60,61,62,63,64';
    SET @i = 1;
    SET @len = LENGTH(@menu_ids) - LENGTH(REPLACE(@menu_ids, ',', '')) + 1;
    WHILE @i <= @len DO
        SET @menu_id = SUBSTRING_INDEX(SUBSTRING_INDEX(@menu_ids, ',', @i), ',', -1);
INSERT INTO sys_permission_menus (created_at, permission_id, menu_id) VALUES (NOW(), 3, @menu_id);
SET @i = @i + 1;
END WHILE;
END //
CALL batch_insert_perm_menu2() //
DROP PROCEDURE IF EXISTS batch_insert_perm_menu2 //

    -- ==============================================
-- 10. 插入默认角色
-- ==============================================
    INSERT INTO sys_roles(id, tenant_id, sort_order, name, code, status, is_protected, is_system, description, created_at)
    VALUES (1, 0, 1, '平台管理员', 'platform:admin', 'ON', true, true, '拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源', NOW()),
    (2, 0, 2, '租户管理员模板', 'template:tenant:manager', 'ON', true, true, '租户管理员角色，拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源', NOW());
ALTER TABLE sys_roles AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_roles);

-- ==============================================
-- 11. 插入角色元数据
-- ==============================================
INSERT INTO sys_role_metadata(id, tenant_id, role_id, is_template, scope, sync_policy, created_at)
VALUES (1, 0, 1, false, 'PLATFORM', 'AUTO', NOW()),
       (2, 0, 2, true, 'TENANT', 'AUTO', NOW());
ALTER TABLE sys_role_metadata AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_role_metadata);

-- ==============================================
-- 12. 插入角色-权限关联
-- ==============================================
INSERT INTO sys_role_permissions (created_at, role_id, permission_id)
SELECT NOW(), 1, id FROM sys_permissions WHERE id IN (1,2,4);
INSERT INTO sys_role_permissions (created_at, role_id, permission_id)
SELECT NOW(), 2, id FROM sys_permissions WHERE id IN (1,3);

-- ==============================================
-- 13. 插入用户-租户关联关系
-- ==============================================
INSERT INTO sys_memberships (id, tenant_id, user_id, org_unit_id, position_id, role_id, is_primary, status)
VALUES (1, 0, 1, null, null, 1, true, 'ACTIVE');
ALTER TABLE sys_memberships AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_memberships);

-- ==============================================
-- 14. 插入租户成员-角色关联关系（修复原脚本少逗号错误）
-- ==============================================
INSERT INTO sys_membership_roles (id, membership_id, tenant_id, role_id, is_primary, status)
VALUES (1, 1, 0, 1, true, 'ACTIVE');
ALTER TABLE sys_membership_roles AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_membership_roles);

-- ==============================================
-- 15. 插入后台菜单/目录（JSON字段meta直接适配MySQL）
-- ==============================================
INSERT INTO sys_menus(id, parent_id, type, name, path, redirect, component, status, created_at, meta)
VALUES (1, null, 'CATALOG', 'Dashboard', '/dashboard', null, 'BasicLayout', 'ON', NOW(),
        '{"order":-1, "title":"page.dashboard.title", "icon":"lucide:layout-dashboard", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (2, 1, 'MENU', 'Analytics', 'analytics', null, 'dashboard/analytics/index.vue', 'ON', NOW(),
        '{"order":-1, "title":"page.dashboard.analytics", "icon":"lucide:area-chart", "affixTab": true, "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (10, null, 'CATALOG', 'TenantManagement', '/tenant', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2000, "title":"menu.tenant.moduleName", "icon":"lucide:building-2", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (11, 10, 'MENU', 'TenantMemberManagement', 'tenants', null, 'app/tenant/tenant/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.tenant.member", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (20, null, 'CATALOG', 'OrganizationalPersonnelManagement', '/opm', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2001, "title":"menu.opm.moduleName", "icon":"lucide:users", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (21, 20, 'MENU', 'OrgUnitManagement', 'org-units', null, 'app/opm/org_unit/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.opm.orgUnit", "icon":"lucide:building-2", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (22, 20, 'MENU', 'PositionManagement', 'positions', null, 'app/opm/position/index.vue', 'ON', NOW(),
        '{"order":3, "title":"menu.opm.position", "icon":"lucide:briefcase", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (23, 20, 'MENU', 'UserManagement', 'users', null, 'app/opm/users/index.vue', 'ON', NOW(),
        '{"order":4, "title":"menu.opm.user", "icon":"lucide:users", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (24, 20, 'MENU', 'UserDetail', 'users/detail/:id', null, 'app/opm/users/detail/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.opm.userDetail", "icon":"", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":true, "hideInTab":false}'),

       (30, null, 'CATALOG', 'PermissionManagement', '/permission', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2002, "title":"menu.permission.moduleName", "icon":"lucide:shield-check", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (31, 30, 'MENU', 'PermissionPointsManagement', 'permissions', null, 'app/permission/permission/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.permission.permission", "icon":"lucide:shield-ellipsis", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (32, 30, 'MENU', 'RoleManagement', 'roles', null, 'app/permission/role/index.vue', 'ON', NOW(),
        '{"order":2, "title":"menu.permission.role", "icon":"lucide:shield-user", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (33, 30, 'MENU', 'MenuManagement', 'menus', null, 'app/permission/menu/index.vue', 'ON', NOW(),
        '{"order":3, "title":"menu.permission.menu", "icon":"lucide:square-menu", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (34, 30, 'MENU', 'APIManagement', 'apis', null, 'app/permission/api/index.vue', 'ON', NOW(),
        '{"order":4, "title":"menu.permission.api", "icon":"lucide:route", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (40, null, 'CATALOG', 'InternalMessageManagement', '/internal-message', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2003, "title":"menu.internalMessage.moduleName", "icon":"lucide:mail", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (41, 40, 'MENU', 'InternalMessageList', 'messages', null, 'app/internal_message/message/index.vue', 'ON', NOW(),
        '{"order": 1, "title":"menu.internalMessage.internalMessage", "icon":"lucide:message-circle-more", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (42, 40, 'MENU', 'InternalMessageCategoryManagement', 'categories', null,
        'app/internal_message/category/index.vue', 'ON', NOW(),
        '{"order":2, "title":"menu.internalMessage.internalMessageCategory", "icon":"lucide:calendar-check", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (50, null, 'CATALOG', 'LogAuditManagement', '/log', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2004, "title":"menu.log.moduleName", "icon":"lucide:activity", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (51, 50, 'MENU', 'LoginAuditLog', 'login-audit-logs', null, 'app/log/login_audit_log/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.log.loginAuditLog", "icon":"lucide:user-lock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (52, 50, 'MENU', 'ApiAuditLog', 'api-audit-logs', null, 'app/log/api_audit_log/index.vue', 'ON', NOW(),
        '{"order":2, "title":"menu.log.apiAuditLog", "icon":"lucide:file-clock", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),

       (60, null, 'CATALOG', 'System', '/system', null, 'BasicLayout', 'ON', NOW(),
        '{"order":2005, "title":"menu.system.moduleName", "icon":"lucide:settings", "keepAlive":true, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (61, 60, 'MENU', 'DictManagement', 'dict', null, 'app/system/dict/index.vue', 'ON', NOW(),
        '{"order":1, "title":"menu.system.dict", "icon":"lucide:library-big", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (62, 60, 'MENU', 'FileManagement', 'files', null, 'app/system/files/index.vue', 'ON', NOW(),
        '{"order":2, "title":"menu.system.file", "icon":"lucide:file-search", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (63, 60, 'MENU', 'TaskManagement', 'tasks', null, 'app/system/task/index.vue', 'ON', NOW(),
        '{"order":3, "title":"menu.system.task", "icon":"lucide:list-todo", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}'),
       (64, 60, 'MENU', 'LoginPolicyManagement', 'login-policies', null,
        'app/system/login_policy/index.vue', 'ON', NOW(),
        '{"order":5, "title":"menu.system.loginPolicy", "icon":"lucide:shield-x", "keepAlive":false, "hideInBreadcrumb":false, "hideInMenu":false, "hideInTab":false}');
ALTER TABLE sys_menus AUTO_INCREMENT = (SELECT MAX(id) + 1 FROM sys_menus);

-- 事务提交+恢复外键检查+还原语句结束符
COMMIT;
SET FOREIGN_KEY_CHECKS = 1;
DELIMITER ;
