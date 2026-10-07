# codex-reset-watcher

这是一个 Go 实现的 **@thsottiaux 帖子重置消息分类器（v0.1.0）**。把已获取的帖子原文交给程序，它直接调用 `https://sub2api.yjkj02.com/v1/chat/completions` 判断是否为 Codex / ChatGPT Work 额度重置消息。

> **当前不是自动监控服务**：不会抓取 X 帖子、后台轮询、保存游标或发送通知。Docker Compose 仅提供按需运行的容器，不能用 `docker compose up -d` 代替持续监控。X 来源认证、增量采集和告警属于后续工作。

## 分类结果

| 分类 | 含义 |
| --- | --- |
| `COMPLETED` | 原帖明确宣布额度重置已处理或完成；**不等于你的账户已到账** |
| `SCHEDULED` | 明确预告未来的额度重置 |
| `BANKED` | 发放储备重置，可能需要自行领取或使用 |
| `ISSUE` | 重置未覆盖、延迟或仍在调查 |
| `HINT` | 条件承诺、投票、暗示等，不能当成完成通知 |
| `UNRELATED` | 与额度重置无关 |
| `REVIEW_REQUIRED` | 语义不确定，需要人工核对 |

输出 JSON 包含 `category`、`evidence`（原帖中的精确子串）、`reason`、`scope`、`audience`。`scope` 和 `audience` 只是模型提取的描述，不是账户权益证明。接口出错、返回不完整、JSON 格式错误、证据不在当前原帖中时，程序输出 `REVIEW_REQUIRED` 错误并以非零码退出；不会把失败当成“没有新帖子”。输入仅供分类，程序本身没有验证帖子是否真的由该账号发布。

## 配置

| 配置 | 默认值 | 用途 |
| --- | --- | --- |
| `SUB2API_BASE_URL` | `https://sub2api.yjkj02.com` | 仅允许 HTTPS origin；程序追加 `/v1/chat/completions` |
| `SUB2API_MODEL` | `gpt-5.6-sol` | 识别模型；更换后需验证当前 API 账号是否能调用 |
| `SUB2API_API_KEY` | 无，必填 | sub2api Bearer Key；不可提交到 Git |

本地 CLI 还兼容环境变量 `HERMES_CUSTOM_SUB2API_YJKJ02_COM_API_KEY` 作为密钥回退来源；Compose **只**传入 `SUB2API_API_KEY`。不要把真实密钥写进 `compose.yaml`、构建参数或日志。

## Docker Compose（单次分类）

准备一份不跟踪的 `.env`，从 `.env.example` 复制后设置 `SUB2API_API_KEY`，也可通过宿主环境变量注入。`.env` 被 `.gitignore` 和 `.dockerignore` 排除。Compose 的 `SUB2API_MODEL` 可直接在 `.env` 中修改。注意 Docker 管理员能看到容器环境变量，部署机访问权限应受控。

```bash
# 在仓库目录中
docker compose build classifier
docker compose run --rm classifier -id 2107676072871600470 \
  -text 'Therefore ... the reset has been processed. Enjoy!'
```

需要父帖语境时加 `-parent '父帖原文'`。Compose 使用 `manual` profile，让普通 `docker compose up -d` 不会误将一次性 CLI 当作守护程序。当前镜像没有暴露端口、定时器或告警接口。**未验证 Docker Hub 镜像前，不要假定有公开可拉取的镜像**；此处由本地源码构建。

## 本机运行与验证

需要 Go 1.23 或更高版本。

```bash
export SUB2API_API_KEY='由部署环境安全注入的密钥'
go run . -id 2107676072871600470 \
  -text 'Therefore ... the reset has been processed. Enjoy!'
go test ./...
go vet ./...
```

预期这条帖子被分类为 `COMPLETED`，但模型回复文本可能随服务版本变化。`-id` 是审计用的输入值，并非程序从 X 校验的 ID；输入内容和父帖会发送给所配置的 API 服务。错误时请查看退出状态；不要把失败或不确定结果当成明确重置。

## 发布范围与后续

`v0.1.0` 仅包含 Go 分类器、测试、Dockerfile 与手动执行的 Compose 配置。要成为真正的 watcher，下一步需接入 X 官方 API 获取并校验账号 ID 和作者，包含回复、分页及增量游标，落盘后再推进游标；随后加入事件去重、告警渠道和采集失败健康告警。没有 X 权限或通知目标时，不能声称这些功能已上线。
