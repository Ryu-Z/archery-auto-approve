# archery-auto-approve

[![CI](https://github.com/Ryu-Z/archery-auto-approve/actions/workflows/ci.yml/badge.svg)](https://github.com/Ryu-Z/archery-auto-approve/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/Ryu-Z/archery-auto-approve)](https://github.com/Ryu-Z/archery-auto-approve/releases)

`archery-auto-approve` 是一个基于 Go 1.21+ 的守护进程，用于在北京时间非人工审批时段自动审批 Archery 待处理工单。

规则如下：

- 周一到周五 `10:00-19:00` 为人工审批时间，不执行自动审批。
- 周一到周五 `00:00-10:00` 和 `19:00-24:00` 自动审批。
- 周六、周日全天自动审批。
- 自动审批备注固定可配置，默认值为 `系统自动审批（非工作时间）`。

## 功能特性

- Viper 读取 `config.yaml`
- 支持 `.env` 注入认证信息
- 固定使用 `Asia/Shanghai` 时区判断自动审批窗口
- 支持 Token 直连认证，或使用用户名/密码调用 `/api/auth/token/` 获取 JWT，并在过期后用 `/api/auth/token/refresh/` 刷新
- 定时轮询待审批工单
- 按 Archery OpenAPI v1.3.0 调用 `GET /api/v1/workflow/` 和 `POST /api/v1/workflow/audit/`
- 审批前会再次查询当前工单状态，已通过则跳过，避免重复审批
- 审批失败自动重试 3 次
- zap JSON 日志
- 并发审批控制
- `SIGINT` / `SIGTERM` 优雅退出
- 可选 `GET /healthz` 健康检查
- 单元测试覆盖时间判断与 API 模拟调用

## 轮询频率

- 默认每 `300` 秒执行一次，也就是每 `5` 分钟轮询一次
- 配置项为 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml) 里的 `poll_interval`
- 如果你想改成每分钟执行一次，可以设置 `poll_interval: 60`

## 项目结构

```text
archery-auto-approve/
├── api/
│   ├── client.go
│   ├── fields.go
│   └── workflow.go
├── config/
│   └── config.go
├── model/
│   └── workflow.go
├── scheduler/
│   └── scheduler.go
├── utils/
│   ├── log.go
│   └── time.go
├── config.yaml
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── main.go
└── README.md
```

## 配置

优先在 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml) 配置基础参数，并把敏感认证信息放在 `.env`。

`config.yaml` 示例：

```yaml
archery:
  base_url: "https://your-archery.example.com"
  token: ""
  refresh_token: ""
  token_ttl: 3600
  auth_scheme: "Bearer"
  workflow_list_path: "/api/v1/workflow/"
  workflow_approve_path: "/api/v1/workflow/audit/"
  workflow_approve_alt: "/api/workflow/approve/"
  token_path: "/api/auth/token/"
  token_refresh_path: "/api/auth/token/refresh/"
  login_path: "/api/v1/user/login/"

poll_interval: 300
log_level: "info"
max_concurrent: 5
approval_remark: "系统自动审批（非工作时间）"
approver: ""
pending_statuses:
  - "workflow_manreviewing"
retry_count: 3
retry_backoff_sec: 2

health:
  enabled: true
  port: 8080
```

`.env` 示例可参考 [\.env.example](/Users/ryuliu/codex/archery-auto-approve/.env.example)：

```dotenv
ARCHERY_AUTO_APPROVE_ARCHERY_BASE_URL=https://your-archery.example.com
ARCHERY_AUTO_APPROVE_ARCHERY_USERNAME=admin
ARCHERY_AUTO_APPROVE_ARCHERY_PASSWORD=xxxxx
ARCHERY_AUTO_APPROVE_ARCHERY_TOKEN=
ARCHERY_AUTO_APPROVE_ARCHERY_REFRESH_TOKEN=
ARCHERY_AUTO_APPROVE_ARCHERY_AUTH_SCHEME=Bearer
ARCHERY_AUTO_APPROVE_ARCHERY_TOKEN_PATH=/api/auth/token/
ARCHERY_AUTO_APPROVE_ARCHERY_TOKEN_REFRESH_PATH=/api/auth/token/refresh/
```

说明：

- 程序启动时会自动读取项目根目录 `.env`。
- 基础运行参数从 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml) 读取，敏感认证信息建议放在 [.env](/Users/ryuliu/codex/archery-auto-approve/.env)。
- 如果已持有 Token，可直接通过 `.env` 或 `config.yaml` 填写 `archery.token`。
- 如果不填 Token，则会使用 `.env` 中的 `username/password` 调用你提供的 `/api/auth/token/`，从返回体中提取 `access` 作为审批请求的 Bearer Token。
- 如果 `access token` 失效且已拿到 `refresh`，程序会先调用 `/api/auth/token/refresh/` 续期。
- 如果 JWT 接口不可用，仍会回退到 `login_path`，并兼容旧的 `{ "token": "..." }` 返回结构。
- 根据 [openapi.json](/Users/ryuliu/Desktop/openapi.json)，待审批状态默认使用 `workflow_manreviewing`。
- 当前工单列表查询只支持 `/api/v1/workflow/`，不再兼容旧版 `/api/workflow/` 列表接口。
- 根据 [openapi.json](/Users/ryuliu/Desktop/openapi.json)，审批请求里的 `audit_type` 已调整为文档枚举值 `pass`。
- `approver` 留空时，程序会自动使用 `.env` 里的登录用户名作为审批执行人。

## 本地启动

```bash
go run main.go
```

启动后程序会先立即执行一次轮询，之后再按 `poll_interval` 周期继续执行。

如果你已经编译成二进制，也可以这样启动：

```bash
go build -o archery-auto-approve .
./archery-auto-approve
```

## Dockerfile 部署

项目已提供 [Dockerfile](/Users/ryuliu/codex/archery-auto-approve/Dockerfile)，运行镜像基于 `debian:stable-slim`，并且在容器内使用 `app` 用户启动服务。

构建镜像：

```bash
docker build -t archery-auto-approve:latest .
```

启动容器：

```bash
docker run -d \
  --name archery-auto-approve \
  --restart unless-stopped \
  --env-file .env \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -p 8080:8080 \
  archery-auto-approve:latest
```

查看日志：

```bash
docker logs -f archery-auto-approve
```

停止容器：

```bash
docker stop archery-auto-approve
docker rm archery-auto-approve
```

## docker-compose 部署

项目已提供 [docker-compose.yml](/Users/ryuliu/codex/archery-auto-approve/docker-compose.yml)。

启动：

```bash
docker compose up -d --build
```

查看日志：

```bash
docker compose logs -f
```

停止：

```bash
docker compose down
```

`docker-compose.yml` 默认会：

- 使用当前目录的 [Dockerfile](/Users/ryuliu/codex/archery-auto-approve/Dockerfile) 构建镜像
- 读取当前目录的 [.env](/Users/ryuliu/codex/archery-auto-approve/.env)
- 挂载当前目录的 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml) 到容器内 `/app/config.yaml`
- 暴露健康检查端口 `8080`
- 通过 `/healthz` 自动执行容器健康检查

如果你修改了 [.env](/Users/ryuliu/codex/archery-auto-approve/.env) 或 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml)，需要重新执行：

```bash
docker compose up -d --build
```

## 日志示例

```json
{"level":"info","ts":"2026-03-20T21:45:00+08:00","msg":"polling workflows","now":"2026-03-20T21:45:00+08:00","auto_approve":true}
{"level":"info","ts":"2026-03-20T21:45:01+08:00","msg":"pending workflows loaded","count":3}
{"level":"info","ts":"2026-03-20T21:45:02+08:00","msg":"workflow approved","workflow_id":123,"name":"上线工单-订单表DDL","remark":"系统自动审批（非工作时间）"}
```

## systemd 示例

```ini
[Unit]
Description=Archery Auto Approve
After=network.target

[Service]
WorkingDirectory=/opt/archery-auto-approve
ExecStart=/opt/archery-auto-approve/archery-auto-approve
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

修改完 [config.yaml](/Users/ryuliu/codex/archery-auto-approve/config.yaml) 或 [.env](/Users/ryuliu/codex/archery-auto-approve/.env) 后，需要重启进程或容器让配置生效。

## 健康检查

本服务默认暴露 `GET /healthz`，返回 `200 OK` 时表示进程正常运行。

本地检查：

```bash
curl http://127.0.0.1:8080/healthz
```

查看 `docker compose` 健康状态：

```bash
docker compose ps
```

如果容器状态显示为 `healthy`，说明 `/healthz` 探针通过。

## Kubernetes CronJob 说明

如果希望以周期任务运行，也可以将镜像部署为 CronJob。但如果需要持续暴露 `/healthz` 和保持 Token/连接复用，推荐直接使用 Deployment 或 systemd 常驻运行。

## 测试

```bash
go test ./...
```
