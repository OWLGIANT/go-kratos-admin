package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	pagination "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-admin/app/admin/service/internal/data"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"

	"go-wind-admin/pkg/middleware/auth"
	"go-wind-admin/pkg/utils/converter"
)

type PermissionService struct {
	adminV1.PermissionServiceHTTPServer

	log *log.Helper

	permissionRepo  *data.PermissionRepo
	menuRepo        *data.MenuRepo
	apiResourceRepo *data.ApiResourceRepo

	membershipRepo *data.MembershipRepo

	authorizer *data.Authorizer

	menuPermissionConverter *converter.MenuPermissionConverter
	apiPermissionConverter  *converter.ApiPermissionConverter
}

func NewPermissionService(
	ctx *bootstrap.Context,
	permissionRepo *data.PermissionRepo,
	membershipRepo *data.MembershipRepo,
	menuRepo *data.MenuRepo,
	apiResourceRepo *data.ApiResourceRepo,
	authorizer *data.Authorizer,
) *PermissionService {
	svc := &PermissionService{
		log:                     ctx.NewLoggerHelper("permission/service/admin-service"),
		permissionRepo:          permissionRepo,
		membershipRepo:          membershipRepo,
		menuRepo:                menuRepo,
		apiResourceRepo:         apiResourceRepo,
		authorizer:              authorizer,
		menuPermissionConverter: converter.NewMenuPermissionConverter(),
		apiPermissionConverter:  converter.NewApiPermissionConverter(),
	}

	svc.init()

	return svc
}

func (s *PermissionService) init() {
	ctx := context.Background()
	if count, _ := s.permissionRepo.Count(ctx, []func(s *sql.Selector){}); count == 0 {
		_, _ = s.SyncApiResources(ctx, &emptypb.Empty{})
		_, _ = s.SyncMenus(ctx, &emptypb.Empty{})
	}
}

func (s *PermissionService) List(ctx context.Context, req *pagination.PagingRequest) (*adminV1.ListPermissionResponse, error) {
	return s.permissionRepo.List(ctx, req)
}

func (s *PermissionService) Get(ctx context.Context, req *adminV1.GetPermissionRequest) (*adminV1.Permission, error) {
	return s.permissionRepo.Get(ctx, req)
}

func (s *PermissionService) Create(ctx context.Context, req *adminV1.CreatePermissionRequest) (*emptypb.Empty, error) {
	if req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.Data.CreatedBy = trans.Ptr(operator.UserId)

	if err = s.permissionRepo.Create(ctx, req); err != nil {
		return nil, err
	}

	// 重置权限策略
	if err = s.authorizer.ResetPolicies(ctx); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PermissionService) Update(ctx context.Context, req *adminV1.UpdatePermissionRequest) (*emptypb.Empty, error) {
	if req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.Data.UpdatedBy = trans.Ptr(operator.UserId)

	if err = s.permissionRepo.Update(ctx, req); err != nil {
		return nil, err
	}

	// 重置权限策略
	if err = s.authorizer.ResetPolicies(ctx); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PermissionService) Delete(ctx context.Context, req *adminV1.DeletePermissionRequest) (*emptypb.Empty, error) {
	if err := s.permissionRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	// 重置权限策略
	if err := s.authorizer.ResetPolicies(ctx); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PermissionService) SyncApiResources(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 清理 API 相关权限
	_ = s.permissionRepo.CleanApiPermissions(ctx)

	// 查询所有启用的 API 资源
	apis, err := s.apiResourceRepo.List(ctx, &pagination.PagingRequest{
		NoPaging: trans.Ptr(true),
		Query:    trans.Ptr(`{"status":"ON"}`),
		OrderBy:  []string{"operation"},
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(apis.Items, func(i, j int) bool {
		a, b := apis.Items[i], apis.Items[j]
		if a.GetModule() != b.GetModule() {
			return a.GetModule() < b.GetModule()
		}
		if a.GetPath() != b.GetPath() {
			return a.GetPath() > b.GetPath()
		}
		return a.GetOperation() > b.GetOperation()
	})

	rootID, _ := s.insertApiRootNode(ctx)

	var permissions []*adminV1.Permission
	codeMap := make(map[string]struct{})
	for _, api := range apis.Items {
		code := s.apiPermissionConverter.ConvertCodeByOperationID(api.GetOperation())
		if code == "" {
			continue
		}

		if _, exists := codeMap[code]; exists {
			code = s.apiPermissionConverter.ConvertCodeByPath(api.GetMethod(), api.GetPath())
			if code == "" {
				continue
			}
			if _, exists = codeMap[code]; exists {
				s.log.Warnf("SyncApiResources: duplicate permission code for API %s - %s, skipped", api.GetOperation(), code)
				continue
			}
		}

		codeMap[code] = struct{}{}

		permission := &adminV1.Permission{
			Name:     api.Description,
			Module:   api.Module,
			Code:     trans.Ptr(code),
			Type:     trans.Ptr(adminV1.Permission_API),
			Status:   trans.Ptr(adminV1.Permission_ON),
			ParentId: trans.Ptr(rootID),
			Bind:     &adminV1.Permission_ApiResourceId{ApiResourceId: api.GetId()},
		}
		permissions = append(permissions, permission)

		//s.log.Debugf("SyncApiResources: prepared permission for API %s - %s", api.GetOperation(), code)
	}

	if err = s.permissionRepo.BatchCreate(ctx, operator.GetTenantId(), permissions); err != nil {
		s.log.Errorf("batch create api permissions failed: %s", err.Error())
		return nil, err
	}

	createdPermissions, err := s.permissionRepo.ListApiPermissions(ctx)
	if err != nil {
		return nil, err
	}

	// 构建 code -> createdPermission 映射
	codeToCreatedPerm := make(map[string]*adminV1.Permission, len(createdPermissions))
	for _, cp := range createdPermissions {
		if cp.GetCode() != "" {
			codeToCreatedPerm[cp.GetCode()] = cp
		}
	}

	parentIDs := make(map[uint32]uint32)

	if err = s.permissionRepo.UpdateParentIDs(ctx, parentIDs); err != nil {
		s.log.Errorf("batch update permission parent IDs failed: %s", err.Error())
		return nil, err
	}

	createdPermissions, err = s.permissionRepo.ListApiPermissions(ctx)
	if err != nil {
		return nil, err
	}

	paths := s.buildPermissionIDPaths(createdPermissions)
	//s.log.Info("Built permission ID paths for menu permissions", paths)

	if err = s.permissionRepo.UpdatePaths(ctx, paths); err != nil {
		s.log.Errorf("batch update permission parent IDs failed: %s", err.Error())
		return nil, err
	}

	// 重置权限策略
	if err = s.authorizer.ResetPolicies(ctx); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PermissionService) SyncMenus(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 清理菜单相关权限
	_ = s.permissionRepo.CleanMenuPermissions(ctx)

	// 查询所有启用的 API 资源
	menus, err := s.menuRepo.List(ctx, &pagination.PagingRequest{
		NoPaging: trans.Ptr(true),
		Query:    trans.Ptr(`{"status":"ON"}`),
		OrderBy:  []string{"-id"},
	}, false)
	if err != nil {
		return nil, err
	}

	s.menuPermissionConverter.ComposeMenuPaths(menus.Items)

	sort.SliceStable(menus.Items, func(i, j int) bool {
		return menus.Items[i].GetParentId() < menus.Items[j].GetParentId()
	})

	parentModules := make(map[uint32]string)
	menuIDToCode := make(map[uint32]string, len(menus.Items))
	var permissions []*adminV1.Permission
	for _, menu := range menus.Items {
		code := s.menuPermissionConverter.ConvertCode(menu.GetPath(), menu.GetType())
		if code == "" {
			continue
		}

		perType := s.menuPermissionConverter.MenuTypeToPermissionType(menu.GetType())

		parentModule := parentModules[menu.GetParentId()]
		if menu.ParentId == nil {
			parentModule = menu.GetName()
		}

		permission := &adminV1.Permission{
			Name:   menu.Name,
			Code:   trans.Ptr(code),
			Type:   trans.Ptr(perType),
			Status: trans.Ptr(adminV1.Permission_ON),
			Bind:   &adminV1.Permission_MenuId{MenuId: menu.GetId()},
		}
		if parentModule != "" {
			permission.Module = trans.Ptr(parentModule)
		}
		permissions = append(permissions, permission)

		menuIDToCode[menu.GetId()] = code

		if menu.ParentId == nil {
			parentModules[menu.GetId()] = menu.GetName()
			//s.log.Infof("Menu ID %d set as parent module: %s", menu.GetId(), menu.GetName())
		}
	}

	if err = s.permissionRepo.BatchCreate(ctx, operator.GetTenantId(), permissions); err != nil {
		s.log.Errorf("batch create api permissions failed: %s", err.Error())
		return nil, err
	}

	createdPermissions, err := s.permissionRepo.ListMenuPermissions(ctx)
	if err != nil {
		return nil, err
	}

	// 构建 code -> createdPermission 映射
	codeToCreatedPerm := make(map[string]*adminV1.Permission, len(createdPermissions))
	for _, cp := range createdPermissions {
		if cp.GetCode() != "" {
			codeToCreatedPerm[cp.GetCode()] = cp
		}
	}

	// 遍历原始 menus，把 menu.ParentId 映射为对应 permission 的 ParentId，并逐条更新
	parentIDs := make(map[uint32]uint32)
	for _, menu := range menus.Items {
		menuID := menu.GetId()
		code, ok := menuIDToCode[menuID]
		if !ok {
			continue
		}
		createdPerm := codeToCreatedPerm[code]
		if createdPerm == nil {
			continue
		}

		parentMenuID := menu.GetParentId()
		if parentMenuID == 0 {
			// 根节点，确保 permission 的 ParentId 为 nil（可选：如果需要显式清空）
			continue
		}

		parentCode, ok := menuIDToCode[parentMenuID]
		if !ok {
			continue
		}
		parentCreatedPerm := codeToCreatedPerm[parentCode]
		if parentCreatedPerm == nil {
			continue
		}

		parentIDs[createdPerm.GetId()] = parentCreatedPerm.GetId()
	}

	if err = s.permissionRepo.UpdateParentIDs(ctx, parentIDs); err != nil {
		s.log.Errorf("batch update permission parent IDs failed: %s", err.Error())
		return nil, err
	}

	createdPermissions, err = s.permissionRepo.ListMenuPermissions(ctx)
	if err != nil {
		return nil, err
	}

	paths := s.buildPermissionIDPaths(createdPermissions)
	//s.log.Info("Built permission ID paths for menu permissions", paths)

	if err = s.permissionRepo.UpdatePaths(ctx, paths); err != nil {
		s.log.Errorf("batch update permission parent IDs failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// buildPermissionIDPaths 将 permissions 转换为 id -> 树形 id 路径映射（格式 "/1/2/"）。
// 行为：
// - 构建 id->menu 映射加速查找。
// - 使用递归 + 记忆化计算每个节点的路径。
// - 若父节点为 0、缺失或出现循环引用，则将该节点视为根，路径为 "/<id>"。
func (s *PermissionService) buildPermissionIDPaths(permissions []*adminV1.Permission) map[uint32]string {
	// 1. 空输入防御
	if len(permissions) == 0 {
		//s.log.Warn("buildPermissionIDPaths: empty permissions list")
		return make(map[uint32]string)
	}

	// 2. 建立ID->Permission映射（过滤无效节点）
	mByID := make(map[uint32]*adminV1.Permission, len(permissions))
	validIDs := make(map[uint32]struct{}, len(permissions)) // 记录所有有效ID，用于快速校验
	for _, p := range permissions {
		if p == nil {
			//s.log.Warn("buildPermissionIDPaths: skip nil permission")
			continue
		}
		id := p.GetId()
		if id == 0 {
			//s.log.Warn("buildPermissionIDPaths: skip permission with zero ID")
			continue
		}
		mByID[id] = p
		validIDs[id] = struct{}{}
	}

	// 3. 记忆化缓存（仅缓存最终路径，避免递归过程中提前写入）
	idPathMemo := make(map[uint32]string, len(mByID))

	// build 递归构建路径：核心修复父节点匹配+缓存逻辑
	var build func(uint32, map[uint32]bool) string
	build = func(id uint32, visiting map[uint32]bool) string {
		// 基础防御：无效ID直接返回空
		if id == 0 {
			//s.log.Debug("buildPermissionIDPaths: skip zero ID")
			return ""
		}

		// 缓存命中：直接返回（核心：仅返回已完成的路径）
		if path, ok := idPathMemo[id]; ok {
			//s.log.Debugf("buildPermissionIDPaths: cache hit for ID %d: %s", id, path)
			return path
		}

		// 节点不存在：返回空
		perm, ok := mByID[id]
		if !ok || perm == nil {
			//s.log.Warnf("buildPermissionIDPaths: permission ID %d not found or nil", id)
			return ""
		}

		// 循环引用防御：发现循环则视为根节点
		if visiting[id] {
			path := fmt.Sprintf("/%d/", id)
			idPathMemo[id] = path // 缓存循环节点路径
			//s.log.Warnf("buildPermissionIDPaths: detected cycle at ID %d, treating as root", id)
			return path
		}

		parentID := perm.GetParentId()

		// 情况1：根节点（父ID=0、父ID=自身、父ID无效）
		if parentID == 0 || parentID == id {
			path := fmt.Sprintf("/%d/", id)
			idPathMemo[id] = path
			//s.log.Debugf("buildPermissionIDPaths: ID %d is root or has invalid parent, path: %s", id, path)
			return path
		}

		// 情况2：父节点有效，递归构建父路径
		visiting[id] = true // 标记当前节点为“正在访问”，防止循环
		parentPath := build(parentID, visiting)
		delete(visiting, id) // 递归完成后移除标记

		// 父路径有效：拼接子节点路径（核心修复：确保父路径+子ID拼接）
		var finalPath string
		if parentPath != "" {
			// 标准化父路径（确保结尾有/）
			cleanParentPath := strings.TrimSuffix(parentPath, "/") + "/"
			finalPath = fmt.Sprintf("%s%d/", cleanParentPath, id)
			//s.log.Debugf("buildPermissionIDPaths: built path for ID %d: %s", id, finalPath)
		} else {
			// 父路径无效（理论上不会走到这里，因已校验parentID在validIDs）
			finalPath = fmt.Sprintf("/%d/", id)
			//s.log.Debugf("buildPermissionIDPaths: parent path empty for ID %d, set as root: %s", id, finalPath)
		}

		// 写入缓存（仅在路径构建完成后）
		idPathMemo[id] = finalPath
		return finalPath
	}

	// 4. 遍历所有有效节点，构建最终结果
	results := make(map[uint32]string, len(mByID))
	for id := range mByID {
		path := build(id, make(map[uint32]bool)) // 每层递归用独立的循环检测map
		if path != "" {
			results[id] = path
		} else {
		}
	}

	return results
}

// insertApiRootNode 插入 API 资源根节点
func (s *PermissionService) insertApiRootNode(ctx context.Context) (uint32, error) {
	const rootCode = "api:root"

	if err := s.permissionRepo.Update(ctx, &adminV1.UpdatePermissionRequest{
		AllowMissing: trans.Ptr(true),
		Data: &adminV1.Permission{
			Name:   trans.Ptr("API资源根节点"),
			Code:   trans.Ptr(rootCode),
			Type:   trans.Ptr(adminV1.Permission_API),
			Status: trans.Ptr(adminV1.Permission_ON),
		},
	}); err != nil {
		return 0, err
	}

	perm, err := s.permissionRepo.Get(ctx, &adminV1.GetPermissionRequest{
		QueryBy: &adminV1.GetPermissionRequest_Code{Code: rootCode},
	})
	if err != nil {
		return 0, err
	}
	return perm.GetId(), nil
}
