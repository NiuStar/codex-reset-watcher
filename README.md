# codex-reset-watcher

监控 `@thsottiaux` 重置消息：匿名 RSS 读帖，独立公开预览接口核对作者/ID/原文，sub2api 分类，命中 `COMPLETED`/`SCHEDULED`/`BANKED`/`ISSUE` 才向飞书 home 群通知。`COMPLETED` 只是作者宣布处理完成，不保证你的额度到账。`HINT`/`UNRELATED` 不通知，任何来源/分类/发送异常均失败关闭且不推进游标。

## 匿名来源边界

候选 `https://nitter.meowing.monster/thsottiaux/rss` 是**第三方镜像**，不是 X 官方 API。部署前必须在目标服务器上读到该账号的真实近期条目；镜像可能滞后、限流或下线。新帖逐条从 `https://api.fxtwitter.com/thsottiaux/status/<ID>` 交叉核对 ID、作者和正文（该预览接口亦非官方）。feed 需含旧游标以证明没有滚出窗口；否则停机人工核对，不漏报后继续前进。初次运行仅建最新 ID 基线，不补报旧帖。RSS 默认展示有限条目，回复可能不完整；**无法保证与 X 官方时间线等价**，更不能把第三方镜像的 HTTP 200 当作业务完成。

## 私密配置与部署

监控配置存放 `config.json`（示例字段如下，**不得填真密钥到文档**）。项目忽略 `config.json`、`private/` 和 `.env`；Docker 构建上下文也忽略它们。私密文件必须为 `0600`，只读挂载进容器，不写到镜像；在目标机上应由专用用户持有，不应挂载整个 Hermes `.env`。失败时容器退出且 `restart: "no"`，须人工排障后再启动；不应自动反复尝试不确定投递。

```json
{
  "sub2api_base_url": "https://sub2api.yjkj02.com",
  "sub2api_api_key": "<private>",
  "sub2api_model": "gpt-5.6-sol",
  "feishu_app_id": "<private>",
  "feishu_app_secret": "<private>",
  "feishu_home_channel": "<private oc_...>",
  "feishu_domain": "feishu",
  "anonymous_rss_url": "https://nitter.meowing.monster/thsottiaux/rss",
  "anonymous_verify_base_url": "https://api.fxtwitter.com"
}
```

Compose 的 `.env` 仅含 `WATCH_CONFIG_FILE`、`WATCH_DATA_DIR`、`WATCH_UID`、`WATCH_GID`、`WATCH_INTERVAL` 等部署路径/UID；不含密钥。`WATCH_DATA_DIR` 归同一用户所有且 `0700`，一个目录只能运行一个 watcher。先 `docker compose build watcher`，再 `docker compose run --rm watcher -once` 建基线，读回 `state.json`；确认匿名来源持续可用和游标正确后才 `docker compose up -d watcher`。默认间隔 5 分钟。

通知前先落盘 `pending_id`；如果 Feishu 响应丢失，停机待人工核对，不自动重发。取得业务 `code=0` 且 `message_id` 后才推进游标。不要删状态目录重建基线。手动分类模式 `-text` 通过 Compose 的 `classifier` profile 读取同一私密配置，不把 sub2api 密钥放入环境变量或命令行。

`go test ./... && go vet ./...` 为本地回归。`v0.1.0` 仅是手动分类版；`.3` 上已通过一条真实 BANKED 新帖的匿名采集、交叉核对、分类、飞书发送与按 `message_id` 读回（帖 ID `2107913674593644711`）。这只验收了该次事件，不证明第三方源长期无漏帖；原通知将进行中的储备重置误写为“已发放”，后续版本已改为“正在发放（不代表个人账户已有可用额度）”，不会重发旧帖。
