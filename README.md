# codex-reset-watcher

Go 监控程序：轮询 X 官方 API 中 `@thsottiaux` 的公开帖子，经 sub2api 模型分类；发现新发布的重置完成、预告、储备重置或覆盖异常时，向 Hermes 的飞书 home 群发送文本通知。保留 `-text` 手动分类 CLI。`COMPLETED` 仅表示作者宣布处理完成，**不是个人账户已到账的证明**；`HINT`、`UNRELATED` 不推送，`REVIEW_REQUIRED` 或模型失败不推进游标。

## 部署前提

- X 开发者 Bearer Token 能访问用户查询和帖子时间线；没有此凭据不能实时监控。
- `SUB2API_API_KEY` 可用，默认模型 `gpt-5.6-sol`。
- **在 Docker 宿主机上**，Hermes 的 `~/.hermes/.env` 包含 `FEISHU_APP_ID`、`FEISHU_APP_SECRET`、`FEISHU_HOME_CHANNEL=oc_...`，以及 `FEISHU_DOMAIN=feishu`（或 `lark`）。飞书应用须已加入该群并有发消息权限。程序启动时直接读这份文件，不把飞书配置复制进本项目 `.env` 或镜像。Hermes 的飞书 home 群即通知目标；变更 home 群须重启 watcher 生效。
- Docker 宿主机与运行 Hermes 的机器**必须是同一台**，否则 `HERMES_FEISHU_ENV_FILE` 需指定该宿主机上实际存在、经授权访问的 Hermes 配置文件；不得假设本机路径会自动出现在另一台构建/部署机。将整份 Hermes `.env` 只读挂载意味着 watcher 进程有权读取其中其他平台的凭据，应在可信宿主机运行并限制镜像/容器管理权限；不能接受这一权限边界时，应改为独立受限的消息桥，而不是拷贝/提交该文件。
- 状态目录只供**一个** watcher 使用，不可多副本并行。Docker 管理员可见容器的 X/sub2api 环境变量。

```bash
cp .env.example .env
# 只在不跟踪的 .env 中填 X_BEARER_TOKEN、SUB2API_API_KEY；
# HERMES_FEISHU_ENV_FILE 指向 Docker 宿主机 Hermes 用户的 ~/.hermes/.env 绝对路径。
# HERMES_UID/HERMES_GID 填该文件所有者 ID；WATCH_DATA_DIR 填专用绝对路径。
stat -c '%u %g %a' /absolute/path/to/.hermes/.env   # 最后应为 600
mkdir -p /absolute/path/to/private/watcher-state
# 让该目录归同一 UID/GID 所有，权限设为 700；不要修改 Hermes .env 的权限。
chmod 700 /absolute/path/to/private/watcher-state
chmod 600 .env
docker compose build watcher
docker compose run --rm watcher -once  # 首次只建立最新帖子基线，不补发历史消息
docker compose up -d watcher          # 此后持续轮询
```

`WATCH_INTERVAL` 默认为 5 分钟（允许 1m–24h）。首次启动会以 X 最新帖子建立 `since_id`，此前帖子不补发；上线前确认这符合预期，并查看 `WATCH_DATA_DIR/state.json` 中的 ID。后续轮询按 `since_id` 完整翻页、从旧到新处理，每帖落盘游标。回复纳入、转帖排除。

### 故障与恢复

X/分类失败使进程退出并保留游标。准备发送飞书时先保存 `pending_id`；若发送响应丢失或业务结果未确认，**不会盲重试**。先人工检查目标群和原帖，备份 `state.json`，再决定清除 pending（未送达，保留旧 `since_id` 重试）或将 `since_id` 设为该 pending ID（已送达），然后重启。已有 `message_id` 的 pending 会自动恢复而不重发。不要删除状态目录重新建基线，否则会漏掉停机期间的新帖。X API 回溯窗口外的积压不能保证补齐。

```bash
docker compose logs --no-color watcher
# 手动分类无需 X/飞书配置：
docker compose run --rm classifier -id 2107676072871600470 -text 'the reset has been processed'
```

`go test ./... && go vet ./...` 可在源码目录验证。`v0.1.0` 仍是之前的手动分类版本；新监控功能及 Hermes 配置读取尚未发布新镜像或在线实发验证，不能把单元测试当作通知已送达。
