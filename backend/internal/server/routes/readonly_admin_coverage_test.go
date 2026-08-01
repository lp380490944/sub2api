package routes

import (
	"sort"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// reviewedDenyPrefixes 列出【已经过人工审阅、确认对 readonly_admin 拒绝】的管理端模块。
//
// 本测试要求每一条已注册的 admin 路由要么在 middleware 的白名单中、要么匹配本表的
// 某个前缀。新增端点时如果两边都没登记，测试直接失败，强制做出一次显式的安全决策。
//
// 拒绝侧允许用前缀（整块模块被拒时逐条列举无价值）；白名单侧禁止前缀匹配。
// 不对称是有意的：拒绝侧宽松只会让端点保持 403，放行侧宽松会泄露数据。
var reviewedDenyPrefixes = []string{
	"/api/v1/admin/settings",
	"/api/v1/admin/orders",
	"/api/v1/admin/payment",
	"/api/v1/admin/redeem",
	"/api/v1/admin/promo-codes",
	"/api/v1/admin/backup",
	"/api/v1/admin/data-management",
	"/api/v1/admin/audit-logs",
	"/api/v1/admin/proxies",
	"/api/v1/admin/announcements",
	"/api/v1/admin/dashboard",
	"/api/v1/admin/subscriptions",
	"/api/v1/admin/affiliates",
	"/api/v1/admin/channel-monitors",
	"/api/v1/admin/channel-monitor-templates",
	"/api/v1/admin/openai",
	"/api/v1/admin/gemini",
	"/api/v1/admin/antigravity",
	"/api/v1/admin/kiro",
	"/api/v1/admin/grok",
	"/api/v1/admin/metrics",
	"/api/v1/admin/usage",

	// ---- 完全拒绝的模块（无任何端点进入白名单）----
	"/api/v1/admin/api-keys",                 // 客户 API Key 管理
	"/api/v1/admin/compliance",               // 合规确认（含 accept 写操作）
	"/api/v1/admin/error-passthrough-rules",  // 网关错误透传规则（配置面）
	"/api/v1/admin/prompt-audit",             // 提示词审计：客户请求原文
	"/api/v1/admin/risk-control",             // 风控：封禁/解封/命中日志
	"/api/v1/admin/scheduled-test-plans",     // 定时探测计划（会真发上游请求）
	"/api/v1/admin/system",                   // 版本/更新/回滚/重启
	"/api/v1/admin/tls-fingerprint-profiles", // TLS 指纹配置
	"/api/v1/admin/user-attributes",          // 用户属性定义（配置面）

	// ---- 账号 / 分组 / 渠道 / 用户：整模块前缀 ----
	// 这四个模块的集合级写端点路径就是模块根（如 POST /admin/accounts），
	// 而拒绝侧只能按路径前缀匹配，无法在不覆盖整个模块的前提下登记它们。
	// 因此这四条必然是整模块前缀：这些模块中的放行项一律来自白名单的逐条枚举
	// （白名单在本测试中先于拒绝前缀被检查），未列入白名单者运行时一律 403。
	"/api/v1/admin/accounts",
	"/api/v1/admin/channels",
	"/api/v1/admin/groups",
	"/api/v1/admin/users",

	// ---- 运维：按子资源登记，保留新增 ops 子资源时的强制决策 ----
	"/api/v1/admin/ops/advanced-settings",    // PUT 写
	"/api/v1/admin/ops/performance-settings", // PUT 写
	"/api/v1/admin/ops/settings",             // PUT 指标阈值
	"/api/v1/admin/ops/alert-rules",          // 增删改告警规则
	"/api/v1/admin/ops/alert-silences",       // 新建静默
	"/api/v1/admin/ops/alert-events",         // PUT 事件状态
	"/api/v1/admin/ops/email-notification",   // PUT 邮件通知配置（含收件人）
	"/api/v1/admin/ops/errors",               // PUT resolve
	"/api/v1/admin/ops/request-errors",       // PUT resolve
	"/api/v1/admin/ops/upstream-errors",      // PUT resolve
	"/api/v1/admin/ops/runtime",              // PUT/POST 运行时日志与告警开关
	"/api/v1/admin/ops/system-logs",          // POST cleanup
}

func newFullAdminRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	noop := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(
		router.Group("/api/v1"),
		handlers,
		servermiddleware.AdminAuthMiddleware(noop),
		servermiddleware.AuditLogMiddleware(noop),
		servermiddleware.StepUpAuthMiddleware(noop),
		nil,
		nil,
	)
	return router
}

func allowlistSet() map[string]struct{} {
	allow := make(map[string]struct{})
	for _, k := range servermiddleware.ReadonlyAdminAllowlistKeys() {
		allow[k] = struct{}{}
	}
	return allow
}

// TestReadonlyAdminAllowlistIsExhaustive 是"默认拒绝"性质的执行者。
func TestReadonlyAdminAllowlistIsExhaustive(t *testing.T) {
	allow := allowlistSet()
	var unclassified []string
	for _, r := range newFullAdminRouter().Routes() {
		if !strings.HasPrefix(r.Path, "/api/v1/admin") {
			continue
		}
		if _, ok := allow[r.Method+" "+r.Path]; ok {
			continue
		}
		denied := false
		for _, p := range reviewedDenyPrefixes {
			if strings.HasPrefix(r.Path, p) {
				denied = true
				break
			}
		}
		if !denied {
			unclassified = append(unclassified, r.Method+" "+r.Path)
		}
	}
	sort.Strings(unclassified)
	require.Emptyf(t, unclassified,
		"以下 admin 端点既不在 readonlyAdminAllowlist 中，也不在 reviewedDenyPrefixes 中。\n"+
			"请逐条判断该端点对只读管理员是否安全，然后登记到其中一处：\n  %s",
		strings.Join(unclassified, "\n  "))
}

// TestReadonlyAdminAllowlistHasNoStaleEntries 防止白名单残留已删除/改名的路由。
func TestReadonlyAdminAllowlistHasNoStaleEntries(t *testing.T) {
	registered := make(map[string]struct{})
	for _, r := range newFullAdminRouter().Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	var stale []string
	for _, k := range servermiddleware.ReadonlyAdminAllowlistKeys() {
		if _, ok := registered[k]; !ok {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	require.Emptyf(t, stale,
		"白名单中存在未注册的路由（路由被删除或改名后未同步）：\n  %s",
		strings.Join(stale, "\n  "))
}

// TestReadonlyAdminSensitiveEndpointsStayDenied 是敏感端点哨兵。
// 如果哪天有人把这些误加进白名单，本测试立刻红。
func TestReadonlyAdminSensitiveEndpointsStayDenied(t *testing.T) {
	allow := allowlistSet()
	for _, key := range []string{
		"GET /api/v1/admin/accounts/data",                // 导出上游凭据原文
		"POST /api/v1/admin/accounts/data",               // 导入
		"GET /api/v1/admin/proxies/data",                 // 导出代理凭据原文
		"GET /api/v1/admin/groups/:id/api-keys",          // 其他客户的 API Key
		"GET /api/v1/admin/users/:id/api-keys",           // 其他客户的 API Key
		"GET /api/v1/admin/users/:id/balance-history",    // 其他客户的财务流水
		"GET /api/v1/admin/channels/pricing/sync-models", // 伪装成 GET 的上游同步写操作
		"POST /api/v1/admin/accounts",
		"PUT /api/v1/admin/accounts/:id",
		"DELETE /api/v1/admin/accounts/:id",
		"POST /api/v1/admin/accounts/:id/test",
		"POST /api/v1/admin/ops/alert-rules",
		"PUT /api/v1/admin/ops/advanced-settings",
		"PUT /api/v1/admin/ops/performance-settings",
		"POST /api/v1/admin/users/:id/balance",
		"DELETE /api/v1/admin/users/:id",
	} {
		_, found := allow[key]
		require.Falsef(t, found,
			"敏感端点 %q 被加入了 readonly_admin 白名单。这会向只读账号泄露凭据或允许写操作。", key)
	}
}
