# Codex 出站身份仿真（fork 专有）

本文档是 fork `lp380490944/sub2api` 中 **Codex（ChatGPT OAuth）出站身份仿真** 的唯一权威说明：设计宗旨、每一层的出站形态、开关、运维操作与验收方法。上游（Wei-Shaw/sub2api）没有这些能力；同步上游后如有冲突，以本文档的宗旨为准。

参照物有两个，优先级如下：
1. **真实客户端抓包表**（Codex Desktop → 中继 → 网关的二次抓包，2026-09 由运维提供）——决定"形态"
2. **CLIProxyAPI**（`router-for-me/CLIProxyAPI`，`internal/runtime/executor/codex_executor_request.go` 与 `helps/utls_client.go`）——决定"逻辑"

原则：形态以抓包表为准，逻辑以 CPA 为准，不自行发挥；CPA 没做的（rustls ClientHello、HTTP/2 SETTINGS、头顺序、Accept-Encoding）本 fork 也不做。

---

## 0. 为什么需要

生产链路是 **Codex 客户端 → NewAPI 中继 → sub2api → chatgpt.com/backend-api/codex**。中继剥掉全部客户端身份头（sub2api 看到的 UA 是 `Go-http-client/1.1`），只有请求体完整。上游对客户端身份做分桶降载（`server_is_overloaded` / 503）：出站身份越不像真实客户端，越先被甩。

上游 sub2api 的 off 模式出站是：自称 `codex-tui/<ver>`，却**零个**真实 TUI 必发的会话头；反而发两个真实客户端从不发的下划线头（`session_id` / `conversation_id`，16 位 hex）；体内 `prompt_cache_key` 被账号命名空间改成 v4 形状，`session_id ≠ thread_id`，`root_turn_id` / `context_window_id` 原样透传别人的值；TLS/HTTP2 是 Go 默认指纹。2026-09-08 账号 CRM-PRO-20 全天 548 请求 333 次 503 事件。

仿真分三层，每层独立开关、独立灰度：

| 层 | 内容 | 开关 | 状态 |
|---|---|---|---|
| 1 会话身份 | `codex_fingerprint_mode = thread` | 账号 `extra.codex_fingerprint_mode` | 已上线（v0.2.1-ws2） |
| 2 客户端身份 | originator / User-Agent / version 的 OS 形态 | 账号 `credentials.user_agent` | 上游已有机制，本 fork 只定义用法 |
| 3 传输层 | Chrome ClientHello + HTTP/2 + 每请求一连接（CPA 同款） | 账号 `extra.codex_transport = chrome-h2` | 已上线（v0.2.1-ws3） |

---

## 1. 第一层：会话身份（`codex_fingerprint_mode = "thread"`）

代码：`backend/internal/service/openai_codex_fingerprint_thread.go`（派生、原值捕获、存储、去混淆）、`openai_codex_fingerprint.go`（thread 分支）、`repository/gateway_cache.go`（`Get/SetCodexThreadID`）。

### 1.1 出站形态（验收标准）

| 项 | 真实 Codex | thread 模式出站 |
|---|---|---|
| `session-id` / `thread-id` / `x-client-request-id` 头 | 三者相等，v7 | 相等 = 派生 thread |
| `x-codex-parent-thread-id` 头 | 有父线程时发 | 客户端原值存在才发（同规则派生） |
| `session_id` / `conversation_id` 头 | 从不发 | **删除** |
| `x-codex-installation-id` 头 | 每设备 v4 | 账号级 v4（同 device 模式） |
| `x-codex-window-id` 头 | `<thread>:<n>` | `<派生 thread>:<n>`，n 沿用客户端原后缀，无则 `0`（CPA） |
| `x-codex-turn-metadata` 头 | JSON | `{session_id, thread_id, turn_id, root_turn_id, installation_id, window_id, window_number, context_window_id}`，客户端原有其它键保留并同步改写；不注入 `turn_started_at_unix_ms`；总是发送 |
| 体 `prompt_cache_key` | = session | = 派生 thread（缺失则注入） |
| 体 `client_metadata.*` | 与头一致 | session/thread/turn/root_turn/installation/window/turn-metadata 与头逐字段镜像 |
| UUID 版本 | 会话类 v7（48 bit 毫秒时间戳），installation v4 | 同左；原值是 v7 时**沿用原时间戳** |
| 响应体 | 客户端看到自己的 id | 派生值 → 原值 逐对替换（CPA `applyCodexIdentityExposeResponsePayload` 对等） |
| originator / UA / version | 客户端自报 | 不动（见第二层） |

### 1.2 派生规则

- `deriveCodexThreadUUIDv7(seed, kind, original, unixMs)`：前 48 bit = 时间戳；余下由 `HMAC-SHA256(账号 seed, "sub2api:codex-thread-id:v1:" + kind + ":" + 原值)` 填充；版本位 7、变体位 RFC4122。同账号同原值恒等，不同账号必不同（对应 CPA `codexIdentityConfuseUUID`，CPA 输出 v5，这里按抓包表保 v7）。
- 会话输入优先级：`prompt_cache_key` → `client_metadata.thread_id` → 内容种子（与粘性路由同源）。
- 原值是 v7 → 纯函数派生（无状态）；非 v7 → `进程 L1 → Redis (codex_thread_id:<acct>:<hash>, SetNX, 7d) → 首见用 now 派生并落库`；Redis 不可用 → 纪元 2026-01-01 + HMAC 偏移的确定性时间戳（**从不**用 `now()` 生成需要稳定的 id）。
- turn：原值 v7 → 派生保时间戳（重试同 turn 不变）；无 → 每次 `Forward` 一个新 v7。`root_turn_id == turn_id`。
- 原值必须在账号命名空间改写（`applyCodexAccountIdentityClientMetadata*`）**之前**捕获。
- 一份 `codexFingerprintIDs` 走完体、头、响应还原；failover 换账号必重派生（`stageCodexFingerprintIDs` 的 accountID 守卫）。

### 1.3 覆盖路径

`/v1/responses`（非透传 + 透传）、`/v1/chat/completions` 桥、WS v2 头/体（复用 staged IDs）。不覆盖 messages 桥（它自行删/补身份）、compact。

---

## 2. 第二层：客户端身份（originator / User-Agent / version）

上游机制：账号 `credentials.user_agent` → `codexIdentityOverrideUA` → `resolveCodexOutboundIdentity`（`openai_codex_identity.go`）只取"客户端名 / OS / 终端"段，**版本段用同步版本重建**（`openai_codex_client_version_synced`，GitHub releases 每 6h 同步）。

本 fork 约定：四个共享账号不要用同一形态。模板（`0.0.0` 会被替换成同步版本，两处都替换）：

| 形态 | `credentials.user_agent` |
|---|---|
| Ubuntu TUI（默认，不填） | `codex-tui/<ver> (Ubuntu 22.4.0; x86_64) xterm-256color` |
| Mac iTerm（CPA 同款） | `codex-tui/0.0.0 (Mac OS 26.5.1; arm64) iTerm.app/3.6.11 (codex-tui; 0.0.0)` |

```sql
UPDATE accounts SET credentials = jsonb_set(credentials, '{user_agent}',
  '"codex-tui/0.0.0 (Mac OS 26.5.1; arm64) iTerm.app/3.6.11 (codex-tui; 0.0.0)"'), updated_at = now()
WHERE id = <acct>;
-- 回滚：UPDATE accounts SET credentials = credentials - 'user_agent' WHERE id = <acct>;
```

---

## 3. 第三层：传输层（`codex_transport = "chrome-h2"`）

CPA 对 chatgpt.com 的做法（`helps/utls_client.go` `utlsRoundTripper`）：

1. `utls.UClient(conn, &Config{ServerName: host}, utls.HelloChrome_Auto)`——Chrome ClientHello（不是 rustls）；
2. 握手后 `http2.Transport{}.NewClientConn(tlsConn)`，在 uTLS 连接上跑 HTTP/2（x/net/http2 默认 SETTINGS）；
3. 每请求一条新连接，响应体 `Close` 时关连接；
4. 代理 direct / socks5 / http；
5. 不设 Accept-Encoding（http2 自动 gzip）；不处理头顺序。

本 fork 实现（`backend/internal/pkg/tlsfingerprint/chrome_h2.go` + `repository/http_upstream.go` `DoWithTLS` 分支 + `openai_plugin_transport.go` `doOpenAIUpstream`）与 CPA 逐条相同，唯一有意的偏离：

- **建连阶段**（拨号 / 握手 / ALPN 非 h2 / h2 初始化）失败 → `slog.Warn("codex_chrome_h2_fallback")` 并回退现有 `Do`；请求发出后的错误不回退（不重复发非幂等请求）。理由：纽约代理线路质量不可控，不能因传输层试验损失请求。

不改：请求头/体、thread 身份、插件优先级（插件命中时不进传输层）、WS 路径（生产 ws=false）、https 代理（回退现有路径）。

开关：
```sql
UPDATE accounts SET extra = jsonb_set(coalesce(extra,'{}'::jsonb), '{codex_transport}', '"chrome-h2"', true), updated_at = now() WHERE id = <acct>;
-- 回滚：UPDATE accounts SET extra = extra - 'codex_transport' WHERE id = <acct>;
```

已知取舍：HTTP/2 SETTINGS / 伪头顺序仍是 Go 默认（CPA 同）；每请求一次握手经纽约 socks5 约 +150–250ms。

实测证据（2026-09-10，对 `https://tls.peet.ws/api/all`，同一出口）：

| 传输 | JA3 | JA4 | akamai HTTP/2 |
|---|---|---|---|
| Go 默认（现状） | `03117a8ed39ef02427ebbc39f121275c` | `t13d1312h2_f57a46bbacb6_f50d94e863eb` | `b4e6bd27e907d4aa4316619ce615fda4` |
| chrome-h2 | `43d6a32bc32bd4b3c670ff19cc6b151a` | `t13d1516h2_8daaf6152771_d8a2da3f94cd` | `b4e6bd27e907d4aa4316619ce615fda4` |

TLS 指纹变为 Chrome 形态（15 套件 / 16 扩展 / GREASE）；HTTP/2 指纹与 Go 默认相同——这正是 CPA 的边界。

后台：账号编辑弹窗（OpenAI OAuth）「Codex 传输层指纹」下拉；批量编辑 / 新建弹窗未加，用 SQL 或编辑弹窗。

---

## 4. 运维手册

### 4.1 灰度顺序与判定
一次只动一个变量：先账号 11（CRM-PRO-20，历史 503 最重），看美国上午高峰（北京 14–16 点）的 `upstream_status: 503` 比率与首字 p50/p90，再依次 7 → 10 → 2。

### 4.2 开启 thread 模式（seed 已存在则不覆盖）
```sql
UPDATE accounts SET extra = jsonb_set(
  CASE WHEN extra ? 'codex_fingerprint_seed' THEN extra
       ELSE jsonb_set(coalesce(extra,'{}'::jsonb), '{codex_fingerprint_seed}', to_jsonb(gen_random_uuid()::text), true) END,
  '{codex_fingerprint_mode}', '"thread"', true), updated_at = now()
WHERE id = <acct>;
-- 回滚：jsonb_set(extra, '{codex_fingerprint_mode}', '"off"')
```
没有 seed 时模式静默不生效（与 device/session/full 一致）。

### 4.3 观测
- `SUB2API_DEBUG_CODEX_IDENTITY=1`（需重启）：每个出站请求打一行 `[CodexIdentityDebug] where=http|http_passthrough|ws account=… mode=… session-id=… thread-id=… x-client-request-id=… x-codex-window-id=… x-codex-turn-metadata=… session_id="" conversation_id="" body.prompt_cache_key=… body.client_metadata=…`。取头用 `getHeaderRaw`（`Header.Get` 会漏印 wire 大小写的头）。
- 503 事件：`docker compose logs sub2api | grep -E '"upstream_status": 503|currently overloaded'`，按 `account_id` / `model` / 小时聚合。
- 用量：`usage_logs` 中 `output_tokens=0 AND input_tokens=0` 的行 = 流中途失败。

### 4.4 改生产前
先 `SELECT` 读出当前值并落备份文件（`/root/acct<id>-<field>-<ts>.bak.txt`），再改；不要凭前一天的记忆判断当前值（2026-09-10 曾因此把并发 4 误当 15）。

---

## 5. 验收方法

- 单测：`go test ./internal/service/ -run 'Thread|CodexThread'`；端到端 `TestThreadModeE2E_*` 跑完整 `Forward()` 断言 §1.1 全部不变量；**变异检查**：删掉 thread 头分支或删掉"去下划线头"，e2e 必须转红。
- staging：prod dump 副本 + 真实账号（走同一代理）+ `SUB2API_DEBUG_CODEX_IDENTITY=1`，Desktop 形态体（完整 `client_metadata`）与 CLI 形态体（仅 `prompt_cache_key`）各两轮，逐项核对 §1.1；上游 200；探测后核对账号凭据 `expires_at` / token 前缀未变（防 refresh token 轮换）。
- 传输层：本地 h2 TLS 测试服务端抓 `ClientHelloInfo` 断言密码套件 / ALPN 与 `HelloChrome_Auto` 一致；手工对 `https://tls.peet.ws/api/all` 取 JA3 / JA4 / akamai h2 指纹作上线记录。

---

## 6. 变更记录

| 日期 | 版本 | 内容 |
|---|---|---|
| 2026-09-09 | v0.2.1-ws2 | 第一层 thread 模式上线；账号 11 开启 |
| 2026-09-10 | — | 账号 11 设 Mac UA（第二层） |
| 2026-09-10 | v0.2.1-ws3 | 第三层 chrome-h2 传输上线（02:23）；账号 11 开启，走 socks5 → HTTP/2.0 200 |
