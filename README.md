# codex-reset-watcher

Go 监控程序：轮询 X 官方 API 中 `@thsottiaux` 的公开帖子，经 sub2api 的可配置模型分类；发现新发布的重置**完成、预告、储备重置或覆盖异常**时，向指定飞书群发送文本通知。保留手动 `-text` 分类 CLI。`COMPLETED` 表示作者宣布处理完成，**不是个人账户已到账的证明**。`HINT`、`UNRELATED` 不推送；`REVIEW_REQUIRED`/模型失败停止且不推进游标。

## 运行前提

- X 开发者应用的 Bearer Token 能访问用户查询与用户帖子时间线（账户权限及用量/费用以 X 官方为准）；无该凭据就无法实时采集，不能以网页搜索替代。
- 飞书自建应用有发消息权限，已加进目标群，并取得该群 `chat_id`；缺一不可。
- `SUB2API_API_KEY` 可用；默认模型 `gpt-5.6-sol`。仅 `.env` 私下提供密钥，文件不得提交 Git；Docker 管理员可见容器环境变量。
- 稳定持久化 Docker 卷；**一个状态卷只能有一个 watcher 实例**，不能多副本并行。

```bash
cp .env.example .env
# 私下填写 SUB2API_API_KEY、X_BEARER_TOKEN、FEISHU_APP_ID、FEISHU_APP_SECRET、FEISHU_CHAT_ID
chmod 600 .env
docker compose build watcher
docker compose run --rm watcher -once   # 仅建最新帖基线，不追发历史通知
docker compose up -d watcher           # 之后持续轮询；默认每 5 分钟
```

`WATCH_INTERVAL` 可在 `.env` 中配置（1m–24h）。若不想预先建基线，`up -d` 的首次轮询也会建基线。首次启动时以 X 最近一页的最新帖子为 `since_id`，**此前所有帖子都不会补发**；应检查 `/data/state.json` 中的 `user_id`、`since_id`。状态文件通过原子替换落盘；后续每轮以 `since_id` 请求新帖并翻页，全部取完再按时间从旧到新逐条分类/通知/推进游标。X 帖子只验证作者 ID 和可用文本；回复纳入、转帖排除；引用、上下文不足的语义结论应视为模型判断而非账户事实。

示例消息：分类标签、范围/对象（若未知则为空）、原帖证据和 `https://x.com/thsottiaux/status/<ID>`。飞书返回 `code=0` 且 `message_id` 非空才记作成功；非目标帖子也逐条推进游标。

### 失败与恢复

X/分类失败会让进程以非零状态退出；Compose 可重启，游标留在未处理帖子之前。**飞书请求一旦准备发送就先写 `pending_id`**。响应丢失、业务错误或崩溃后，若仍有无 `message_id` 的 `pending_id`，进程会停机，不会盲重发；先人工检查目标群及 `pending_id` 对应原帖，再备份状态文件并作明确恢复决定（若消息已送达，可把 `since_id` 设为该 `pending_id`，清空 `pending_id`/`pending_category` 后再启动；未送达时清空 `pending_id`/`pending_category` 并保留原 `since_id`，下轮重试）。不要随意删掉整个状态卷，否则将重建基线并可能漏掉停机期间的帖子。已有 `message_id` 的 pending 表示发出成功但提交游标中断，程序自动完成恢复，不会再次推送。遇到批量积压或 X 只返回其有限时间窗口内的帖子时，超出 API 可回溯范围的内容无法保证补齐；需人工审计。

```bash
docker compose logs --no-color watcher
# 手动分类仍可用，不需要 X/飞书凭据：
docker compose run --rm classifier -id 2107676072871600470 -text 'the reset has been processed'
```

本机验证：`go test ./... && go vet ./...`。`v0.1.0` 为先前仅支持手动分类的版本；仓库本次改动尚未发布新版本/镜像，也未接入真实 X+飞书凭据做端到端在线验证，切勿把本地测试当作飞书实发。
