# 高频做市、账号管理、托管者管理功能移植指南

## 项目概述

本文档记录了从 beast 项目（Gin+GORM+React）到 go-wind-admin 项目（Kratos+Ent+Vue3）的功能移植工作。

## 已完成的后端工作

### 1. Protobuf API 定义 ✅

#### 数据模型定义
- `backend/api/protos/trading/service/v1/exchange_account.proto` - 交易账号数据模型
- `backend/api/protos/trading/service/v1/server.proto` - 托管者数据模型
- `backend/api/protos/trading/service/v1/hft_market_making.proto` - 高频做市数据模型

#### 服务接口定义
- `backend/api/protos/admin/service/v1/i_exchange_account.proto` - 账号管理服务（15个接口）
- `backend/api/protos/admin/service/v1/i_server.proto` - 托管者管理服务（15个接口）
- `backend/api/protos/admin/service/v1/i_hft_market_making.proto` - 高频做市服务（6个接口）

### 2. Ent Schema 定义 ✅

- `backend/app/admin/service/internal/data/ent/schema/exchange_account.go` - 交易账号表
  - 字段：昵称、交易所、API密钥、密钥（加密）、经纪商ID、备注、绑定IP、限频、账号类型、组合账号等
  - 索引：交易所、原始账号、API密钥（唯一）、账号类型、操作员

- `backend/app/admin/service/internal/data/ent/schema/server.go` - 托管者表
  - 字段：昵称、外网IP、内网IP、端口、机器ID、备注、VPC ID、实例ID、类型、服务器状态信息（JSON）
  - 索引：IP（唯一）、内网IP、类型、操作员、VPC ID

### 3. Repository 层 ✅

#### ExchangeAccountRepo (`exchange_account_repo.go`)
实现的方法：
- `List` - 分页查询账号列表（支持过滤、排序）
- `Get` - 获取单个账号
- `Create` - 创建账号
- `Update` - 更新账号
- `Delete` - 删除账号
- `BatchDelete` - 批量删除
- `Transfer` - 转移账号
- `Search` - 搜索账号
- `UpdateRemark` - 更新备注
- `UpdateBrokerId` - 更新经纪商ID
- `CreateCombined` - 创建组合账号
- `UpdateCombined` - 更新组合账号

#### ServerRepo (`server_repo.go`)
实现的方法：
- `List` - 分页查询托管者列表
- `Get` - 获取单个托管者
- `Create` - 创建托管者
- `BatchCreate` - 批量创建
- `Update` - 更新托管者
- `Delete` - 删除托管者
- `DeleteByIps` - 按IP删除
- `Transfer` - 转移托管者
- `UpdateRemark` - 更新备注
- `UpdateStrategy` - 更新策略
- `GetCanRestartList` - 获取可重启列表

### 4. Service 层 ✅

#### ExchangeAccountService (`exchange_account_service.go`)
- 实现了所有 Protobuf 定义的接口
- 包含敏感信息加密处理的 TODO 标记

#### ServerService (`server_service.go`)
- 实现了所有 Protobuf 定义的接口
- 包含远程操作（重启、日志、停止机器人）的 TODO 标记

#### HftMarketMakingService (`hft_market_making_service.go`)
- 实现了所有 Protobuf 定义的接口
- 包含数据查询和报告生成的 TODO 标记

### 5. 依赖注入配置 ✅

已更新 Wire 配置文件：
- `internal/data/providers/wire_set.go` - 添加了 Repository 构造函数
- `internal/service/providers/wire_set.go` - 添加了 Service 构造函数

## 需要完成的后续工作

### 1. 生成 Ent 代码 ⚠️

由于 ent 工具版本问题，需要手动生成代码：

```bash
cd backend/app/admin/service
# 方法1：使用项目中的 ent 版本
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/data/ent/schema

# 方法2：如果上面失败，尝试更新 ent 版本
go get -u entgo.io/ent/cmd/ent
go run entgo.io/ent/cmd/ent generate ./internal/data/ent/schema
```

生成后会创建以下文件：
- `internal/data/ent/exchangeaccount/` - 账号相关的查询构建器
- `internal/data/ent/server/` - 托管者相关的查询构建器
- 更新 `internal/data/ent/client.go` - 添加新的客户端方法

### 2. 生成 Wire 代码

```bash
cd backend/app/admin/service/cmd/server
go generate
```

这会生成 `wire_gen.go` 文件，包含所有依赖注入的代码。

### 3. 注册 HTTP 路由

需要在 `internal/server/rest.go` 中注册新的服务：

```go
// 在 NewRESTServer 函数中添加
adminV1.RegisterExchangeAccountServiceHTTPServer(srv, exchangeAccountService)
adminV1.RegisterServerServiceHTTPServer(srv, serverService)
adminV1.RegisterHftMarketMakingServiceHTTPServer(srv, hftMarketMakingService)
```

### 4. 数据库迁移

运行应用时，Ent 会自动创建表结构。或者手动执行迁移：

```bash
cd backend/app/admin/service
go run cmd/server/main.go migrate
```

### 5. 实现 TODO 标记的功能

#### ExchangeAccountRepo
- 实现敏感信息加密/解密（SecretKey、PassKey）
- 参考原项目的 `utils.AesEncrypt` 和 `utils.AesDecrypt`

#### ExchangeAccountService
- 在 Create/Update 时加密敏感字段

#### ServerService
- 实现远程服务器操作：
  - `RebootServer` - 调用远程重启接口
  - `GetServerLog` - 从远程服务器获取日志
  - `StopServerRobot` - 停止远程机器人
  - `DeleteServerLog` - 删除远程日志

#### HftMarketMakingService
- 实现数据查询逻辑（需要连接交易数据库或时序数据库）
- 实现报告生成逻辑
- 实现文件下载功能（生成 CSV/Excel 并上传到 OSS）

### 6. 前端移植

#### 6.1 生成前端 API 客户端

```bash
cd backend/api
buf generate
```

这会在 `frontend/apps/admin/src/generated/api/` 目录生成 TypeScript 客户端代码。

#### 6.2 创建前端页面

需要创建以下 Vue3 页面（参考原项目的 React 页面）：

**账号管理页面**
- `frontend/apps/admin/src/views/app/trading/exchange-account/`
  - `index.vue` - 主页面
  - `account-list.vue` - 账号列表
  - `account-drawer.vue` - 账号编辑抽屉
  - `account-view.state.ts` - 状态管理

**托管者管理页面**
- `frontend/apps/admin/src/views/app/trading/server/`
  - `index.vue` - 主页面
  - `server-list.vue` - 托管者列表
  - `server-drawer.vue` - 托管者编辑抽屉
  - `server-view.state.ts` - 状态管理

**高频做市页面**
- `frontend/apps/admin/src/views/app/trading/hft-robots/`
  - `index.vue` - 主页面（已存在，需要更新）
  - `hft-info-list.vue` - HFT 信息列表
  - `midsigexec-orders.vue` - 订单列表
  - `midsigexec-signals.vue` - 信号列表

#### 6.3 创建 Pinia Store

```typescript
// frontend/apps/admin/src/stores/exchange-account.state.ts
export const useExchangeAccountStore = defineStore('exchange-account', () => {
  const service = createExchangeAccountServiceClient(requestClientRequestHandler);

  async function listExchangeAccount(paging?, filter?, orderBy?) {
    return await service.listExchangeAccount({ pagination: paging, filter, orderBy });
  }

  async function createExchangeAccount(values: object) {
    return await service.createExchangeAccount(values);
  }

  // ... 其他方法

  return {
    listExchangeAccount,
    createExchangeAccount,
    // ...
  };
});
```

类似地创建：
- `server.state.ts`
- `hft-market-making.state.ts`

#### 6.4 更新路由配置

在 `frontend/apps/admin/src/router/routes/modules/app/trading.ts` 中添加：

```typescript
{
  path: 'exchange-account',
  name: 'ExchangeAccount',
  meta: {
    icon: 'lucide:wallet',
    title: '账号管理',
    authority: 'trading:exchange-account:list',
  },
  component: () => import('#/views/app/trading/exchange-account/index.vue'),
},
{
  path: 'server',
  name: 'Server',
  meta: {
    icon: 'lucide:server',
    title: '托管者管理',
    authority: 'trading:server:list',
  },
  component: () => import('#/views/app/trading/server/index.vue'),
},
```

### 7. 权限配置

需要在权限管理系统中添加以下权限点：

**账号管理权限**
- `trading:exchange-account:list` - 查看账号列表
- `trading:exchange-account:create` - 创建账号
- `trading:exchange-account:update` - 更新账号
- `trading:exchange-account:delete` - 删除账号
- `trading:exchange-account:transfer` - 转移账号

**托管者管理权限**
- `trading:server:list` - 查看托管者列表
- `trading:server:create` - 创建托管者
- `trading:server:update` - 更新托管者
- `trading:server:delete` - 删除托管者
- `trading:server:reboot` - 重启托管者

**高频做市权限**
- `trading:hft:view` - 查看HFT信息
- `trading:hft:download` - 下载数据

### 8. 测试

#### 8.1 后端测试

```bash
# 启动后端服务
cd backend/app/admin/service
go run cmd/server/main.go

# 测试 API
curl http://localhost:7788/admin/v1/trading/exchange-accounts
curl http://localhost:7788/admin/v1/trading/servers
```

#### 8.2 前端测试

```bash
# 启动前端
cd frontend
pnpm dev

# 访问页面
# http://localhost:5173/trading/exchange-account
# http://localhost:5173/trading/server
# http://localhost:5173/trading/hft-robots
```

## 技术栈对比

| 功能 | Beast 项目 | Go-Wind-Admin 项目 |
|------|-----------|-------------------|
| Web 框架 | Gin | Kratos |
| ORM | GORM | Ent |
| API 定义 | 手写 | Protobuf |
| 前端框架 | React | Vue 3 |
| 状态管理 | Redux | Pinia |
| UI 组件库 | Ant Design | Ant Design Vue (Vben Admin) |
| 构建工具 | Webpack | Vite |

## 架构差异

### Beast 项目架构
```
Controller (Gin Handler)
  → Service (业务逻辑)
    → DAO (GORM 查询)
      → Model (GORM 模型)
```

### Go-Wind-Admin 项目架构
```
HTTP Server (Kratos + Protobuf)
  → Service (业务逻辑)
    → Repository (数据访问接口)
      → Ent Client (ORM 查询)
        → Schema (数据模型)
```

## 关键文件清单

### 后端文件
```
backend/
├── api/protos/
│   ├── trading/service/v1/
│   │   ├── exchange_account.proto
│   │   ├── server.proto
│   │   └── hft_market_making.proto
│   └── admin/service/v1/
│       ├── i_exchange_account.proto
│       ├── i_server.proto
│       └── i_hft_market_making.proto
├── app/admin/service/
│   └── internal/
│       ├── data/
│       │   ├── ent/schema/
│       │   │   ├── exchange_account.go
│       │   │   └── server.go
│       │   ├── exchange_account_repo.go
│       │   ├── server_repo.go
│       │   └── providers/wire_set.go
│       └── service/
│           ├── exchange_account_service.go
│           ├── server_service.go
│           ├── hft_market_making_service.go
│           └── providers/wire_set.go
```

### 前端文件（待创建）
```
frontend/apps/admin/src/
├── views/app/trading/
│   ├── exchange-account/
│   │   ├── index.vue
│   │   ├── account-list.vue
│   │   └── account-drawer.vue
│   ├── server/
│   │   ├── index.vue
│   │   ├── server-list.vue
│   │   └── server-drawer.vue
│   └── hft-robots/
│       └── index.vue (更新)
├── stores/
│   ├── exchange-account.state.ts
│   ├── server.state.ts
│   └── hft-market-making.state.ts
└── router/routes/modules/app/
    └── trading.ts (更新)
```

## 常见问题

### Q1: Ent 代码生成失败
**A:** 检查 ent 版本，确保使用项目兼容的版本。可以尝试：
```bash
go get -u entgo.io/ent@v0.14.5
```

### Q2: Wire 生成失败
**A:** 确保所有 Repository 和 Service 的构造函数签名正确，参数类型匹配。

### Q3: Protobuf 编译错误
**A:** 确保安装了 buf 工具和相关插件：
```bash
brew install bufbuild/buf/buf
```

### Q4: 前端 API 客户端类型错误
**A:** 重新生成前端代码：
```bash
cd backend/api
buf generate
```

## 下一步行动

1. **立即执行**：生成 Ent 代码和 Wire 代码
2. **短期**：实现 TODO 标记的功能，完成后端核心逻辑
3. **中期**：移植前端页面，实现完整的用户界面
4. **长期**：添加单元测试和集成测试

## 联系与支持

如有问题，请参考：
- Kratos 文档：https://go-kratos.dev/
- Ent 文档：https://entgo.io/
- Vben Admin 文档：https://doc.vben.pro/

---

**文档版本**: 1.0
**最后更新**: 2026-02-02
**作者**: Claude Code Assistant
