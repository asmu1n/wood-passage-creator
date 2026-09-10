# Wood Passage Creator

基于 **Echo + Ent + Postgres + Redis** 的 AI 文章生成后端。  
从 `ai-passage-creator` 迁移并重组为 **app（用例）/ module（领域+仓储）/ httpapi（协议）** 分层。

主要能力：用户会话与 VIP、文章三阶段流水线（SSE 进度）、配图（多 Provider + 可选 R2）、Mock 支付、管理端列表与统计概览。

---

## 1. 设计目标（为什么这样拆）

| 目标 | 做法 |
|------|------|
| 业务可扩展 | 用例在 `app/`；领域 + repo 在 `module/`；HTTP 在 `httpapi/api` |
| 依赖清晰 | app/module 依赖 `port`；不直接依赖 Redis/DB 实现细节 |
| 跨域强一致 | `port.WithinTx`（全局，类 logger）+ repo `ClientFrom` |
| 改动半径可控 | HTTP 只依赖 app；agent 流水线暂留 `module/article` |
| 入口干净 | `cmd/server` 只做装配与生命周期 |
| 配置分层 | `config.yml` 主配置；`.env` 仅密钥 |

一句话：**用例与 Request 进 app，领域与 repo 进 module，HTTP 进 httpapi/api，能力进 port，实现进 infra，工具进 pkg，cmd 装配。**

---

## 2. 目录结构

```text
.
├── cmd/server/              # 装配入口（DB/Redis/Tx/路由）
├── docs/
│   ├── api/swagger/         # OpenAPI 生成物
│   ├── SSE_NOTES.md
│   └── REDIS_CACHE.md
├── ent/schema/              # 表结构（改后 go generate）
├── internal/
│   ├── app/                 # ★ 应用用例 + *Request（事务边界在此）
│   ├── module/              # ★ 领域模型 + Repository + repo（article 含 agent）
│   ├── httpapi/             # 协议基建 + api/* Handler（只依赖 app）
│   │   ├── api/             # auth/user/article/payment/statistics
│   │   └── docsui/          # Scalar 文档页
│   ├── port/                # Cache / Locker / ObjectStore / TxManager（含全局 WithinTx）
│   ├── infra/               # DB/Redis/LLM/Image/ObjectStore 等实现
│   ├── pkg/                 # logger、page、response、sse、llmkit
│   └── config/
├── config.yml               # 非密钥配置主文件
├── .env.example             # 仅密钥与 compose 基础设施口令
└── docker-compose*.yml
```

### 各层职责

| 路径 | 职责 |
|------|------|
| `cmd/server` | 组装依赖、`logger.Init`、`database.InitTxManager`、注册 `httpapi` Registrar |
| `internal/app/*` | **全部业务用例**与 HTTP `*Request`；跨 module 写用 `port.WithinTx` |
| `internal/module/*` | 实体、Repository、repo（`ClientFrom`）；**无** Service/HTTP |
| `internal/module/article/agent` | 生成流水线（标题/大纲/正文/配图），暂留 module |
| `internal/httpapi` | middleware、binding、error、health；`api/*` 只调 app |
| `internal/port` | 技术端口（Cache / Locker / ObjectStore / Tx…）；`port.WithinTx` 为全局事务入口 |
| `internal/infra` | port 实现（DB/Redis/LLM/Image/ObjectStore 等） |
| `internal/pkg` | 与领域无关的工具库（logger、page、response、sse…） |

约定见：[`internal/app/README.md`](internal/app/README.md)、[`internal/module/README.md`](internal/module/README.md)、[`docs/SSE_NOTES.md`](docs/SSE_NOTES.md)、[`docs/REDIS_CACHE.md`](docs/REDIS_CACHE.md)、[`docs/TRANSACTIONS.md`](docs/TRANSACTIONS.md)

---

## 3. 依赖方向（必读）

```text
cmd/server
    │
    ▼
 httpapi/api/*  →  app/*  →  module（实体 + Repository）
                      │           │
                      │           └─ repo ── ClientFrom(ctx) ──► 同一 ent Tx
                      └─ port.WithinTx / Cache / …
                                ▲
 infra 实现 port（TxManager、Cache…）
```

**规则：**

1. HTTP **只依赖 app**，不依赖 repo / infra。  
2. app 依赖 module 的 **Repository 接口** 与实体；**不** import `infra`。  
3. 跨 module **强一致写**：app 内 `port.WithinTx` + 多方 repo（同一 ctx）。  
4. module **无** Service；agent 例外地放在 `module/article`。  
5. 避免循环依赖：鉴权在 `httpapi/middleware`。

---

## 4. 请求怎么走（心智模型）

```text
POST /api/article/create
  → httpapi/api/article.Handler
  → Bind app/article.CreateArticleRequest
  → app/article.Service.Create
       → port.WithinTx：扣配额 + 插文章
       → go Phase1（事务外）

POST /api/payment/vip/mock-complete
  → app/payment.CompleteMockVIP
       → port.WithinTx：MarkSucceeded + GrantVIP

GET /api/article/progress/:taskId
  → SSE：app SubscribeProgress → pkg/sse.Hub fan-out
```

---

## 5. 本地开发

### 5.1 依赖

- Go（版本见 `go.mod`）
- Docker（可选，用于 Postgres / Redis）

### 5.2 基础设施

```bash
# 本地默认即可跑：代码默认 localhost + postgres/postgres，与 compose 一致
# 仅启动 Postgres + Redis（本机 go run 使用；宿主端口固定 5432/6379）
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# 可选：容器内全栈（app 也进 compose，需 --profile full）
# docker compose -f docker-compose.yml -f docker-compose.dev.yml --profile full up -d --build

# 需要覆盖密钥 / 远端地址 / 日志时再复制（契约见 .env.example）
# cp .env.example .env
```

**环境变量分层：**

| 场景                         | 需要什么                                                                         |
| ---------------------------- | -------------------------------------------------------------------------------- |
| 本机 `go run` + compose 依赖 | 通常 **不用** `.env`（默认 `DB_HOST/REDIS_HOST=localhost`）                      |
| compose 内 `app`             | compose **写死** `DB_HOST=postgres`、`REDIS_HOST=redis`；账号等来自 env / `.env` |
| 生产 / 预发                  | 注入 `.env.example` 中的运行时项（库账号、主机、Redis、日志等）                  |

开发期几乎不改的宿主端口映射已写死在 `docker-compose.dev.yml`，不再做成 `*_HOST_PORT` 配置项。

### 5.3 运行 API

```bash
# 生产/预发常见覆盖见 .env.example（LOG_LEVEL / ENV / SERVICE_NAME / DB_* / REDIS_* …）
go run ./cmd/server
# 默认 :8080
# 健康检查：http://localhost:8080/health
# API 文档（Scalar）：http://localhost:8080/docs
# Spec 仍由 swag 生成：docs/api/swagger（import: wood-passage-creator/docs/api/swagger）
# SSE 契约：docs/SSE_NOTES.md
#
# 主要 API 前缀 /api ：
#   auth  users  article  payment(vip mock)
# 重新生成文档：
#   swag init -g cmd/server/main.go -o docs/api/swagger --parseDependency --parseInternal
```

访问日志来自 **`httpapi/middleware.AccessLog`**（Echo RequestLogger → `pkg/logger`，event=`http.access`）；业务 / 任务 / 审计同样走 `internal/pkg/logger`（stderr）。HTTP 错误经 **`httpapi.HTTPErrorHandler`** 统一写 JSON。详见 [logger README](internal/pkg/logger/README.md)。

### 5.4 常用命令

```bash
go build ./...
go test ./...   # test/ 包需要本机 Postgres（见 5.2）

# 修改 ent/schema 后重新生成
go generate ./ent

# 业务 Handler 写好 Swagger 注释后重新生成文档
swag init -g cmd/server/main.go -o docs/api/swagger --parseDependency --parseInternal
```

---

## 6. 如何接入一个新功能

### A. 新增业务能力（推荐路径）

1. `module/<name>/`：实体 + `Repository` + `repo`（`ClientFrom`）；**package 用领域名**（如 `user`）。  
2. `app/<name>/`：`*Request` + Service；**package 名与 module 错开**（如 `userapp` / `articleapp`）。  
3. `httpapi/api/<name>/`：Handler + Registrar（只依赖 app）；Swagger 注解直接写 `articleapp.Xxx`、`user.User` 等。  
4. 如需表结构：`ent/schema` + `go generate ./ent`。  
5. `cmd/server`：注入 repo → app，注册 Registrar。  
6. `swag init -g cmd/server/main.go -o docs/api/swagger --parseDependency --parseInternal`。

### B. 在已有能力上加接口

1. `module/<name>`：扩展模型 / Repository（如需）。  
2. `app/<name>`：新增用例方法与 `*Request`。  
3. `httpapi/api/<name>`：Handler + Swagger 注释 + Registrar 挂路由。  

### C. 需要缓存 / 分布式锁 / 对象存储

- 业务侧使用 `port.Cache` / `port.Locker` / `port.ObjectStore`。  
- 实现已在 `infra/cache`、`infra/lock`、`infra/objectstore`；由 `cmd` 装配注入，业务包不 import infra。

---

## 7. 放哪里？快速判定

| 你要加的内容                         | 放哪里                           |
| ------------------------------------ | -------------------------------- |
| 某业务的用例、模型、该业务 API       | `module/<name>/`                 |
| 全局路由挂载、登录态中间件           | `httpapi`                        |
| 「我需要锁/缓存/对象存储，不关心 Redis/R2」 | `port` 接口 + `infra` 实现  |
| 分页、统一 JSON 响应、跨模块基础类型 | `pkg`                            |
| 结构化业务/任务/审计日志             | `pkg/logger`（Service/Job 打点） |
| 仅某一业务用的算法                   | 留在该 `module`，不要进 `pkg`    |
| 表结构                               | `ent/schema`                     |
| 进程启动参数、组装顺序               | `cmd/*`                          |

---

## 8. 协作约定（简）

1. **优先在对应 module 内闭环**；跨模块先谈 Service 接口，避免双向 import 实现细节。  
2. **生成代码**（`ent/*` 非 schema、`docs/api/swagger`）不要手改业务逻辑；改源再生成。  
3. **PR 粒度**：一个业务能力尽量带齐 service + handler + repo（及必要测试），便于评审。  
4. **命名**：新 module 用小写业务名；HTTP 子包可用 `userhttp` 这类包名，避免与 `net/http` 冲突。  
5. **日志**：新写路径用 `logger.Module` + `purpose` + 稳定 `event`；可预期 `BizError` 不打 Error；系统错误由 `httpapi.HTTPErrorHandler` 边界记一次（见 [logger README](internal/pkg/logger/README.md)）。
6. **HTTP**：Handler 成功 `return c.JSON(http.StatusOK, response.OK(data))`；失败 `return err`（`BizError` / bind / validate），由全局错误处理写成统一响应体。

---

## 9. 相关文档索引

| 文档                                                           | 内容                                |
| -------------------------------------------------------------- | ----------------------------------- |
| [internal/module/README.md](internal/module/README.md)         | 业务模块目录约定                    |
| [internal/pkg/README.md](internal/pkg/README.md)               | 公共库边界                          |
| [internal/pkg/logger/README.md](internal/pkg/logger/README.md) | **结构化日志约定与 event 表**       |
| [docs/REDIS_CACHE.md](docs/REDIS_CACHE.md)                     | **项目级 Redis / 缓存策略**（总览） |
| [ent/schema/README.md](ent/schema/README.md)                   | Schema 与 generate 约定             |

有疑问时：先看依赖图（第 3 节）和「放哪里」（第 7 节），再按第 6 节接入第一个 module。
