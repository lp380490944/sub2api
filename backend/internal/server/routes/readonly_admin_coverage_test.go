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

	// ---- 其余整体拒绝的模块 ----
	"/api/v1/admin/api-keys",                 // 客户 API Key 管理
	"/api/v1/admin/compliance",               // 合规确认（含 accept 写操作）
	"/api/v1/admin/error-passthrough-rules",  // 网关错误透传规则（配置面）
	"/api/v1/admin/prompt-audit",             // 提示词审计：客户请求原文
	"/api/v1/admin/risk-control",             // 风控：封禁/解封/命中日志
	"/api/v1/admin/scheduled-test-plans",     // 定时探测计划（会真发上游请求）
	"/api/v1/admin/system",                   // 版本/更新/回滚/重启
	"/api/v1/admin/tls-fingerprint-profiles", // TLS 指纹配置
	"/api/v1/admin/user-attributes",          // 用户属性定义（配置面）
}

// partiallyAllowedModuleRoots 是【存在放行端点】的五个模块根。
//
// 这些模块里既有白名单端点、也有被拒端点，因此它们的拒绝项【必须】逐条登记到
// reviewedDenyExact，绝不能用前缀。理由见 TestReadonlyAdminDenyPrefixesDoNotCoverPartialModules。
var partiallyAllowedModuleRoots = []string{
	"/api/v1/admin/accounts",
	"/api/v1/admin/groups",
	"/api/v1/admin/channels",
	"/api/v1/admin/users",
	"/api/v1/admin/ops",
}

// reviewedDenyExact 列出【位于部分放行模块内部、已人工审阅并确认拒绝】的端点。
//
// 与 reviewedDenyPrefixes 的区别：那张表用于整体拒绝的模块，模块内新增端点默认拒绝
// 即是正确答案，无需人工介入；而本表覆盖的五个模块正是 readonly_admin 唯一能触达的
// 界面，模块内新增端点究竟该放行还是拒绝【必须】有人拍板。因此这里用精确匹配：
// 在这五个模块中新增任何路由，若未登记到白名单或本表，完备性测试立即失败。
//
// 本表长是特性不是缺陷。
var reviewedDenyExact = map[string]struct{}{
	// ---- accounts（51 条）----
	// 上游账号：写操作 / 凭据导出导入 / 触发上游调用
	"POST /api/v1/admin/accounts":                                    {},
	"PUT /api/v1/admin/accounts/:id":                                 {},
	"DELETE /api/v1/admin/accounts/:id":                              {},
	"POST /api/v1/admin/accounts/:id/apply-oauth-credentials":        {},
	"POST /api/v1/admin/accounts/:id/clear-error":                    {},
	"POST /api/v1/admin/accounts/:id/clear-rate-limit":               {},
	"POST /api/v1/admin/accounts/:id/duplicate":                      {},
	"POST /api/v1/admin/accounts/:id/models/sync-upstream":           {},
	"GET /api/v1/admin/accounts/:id/ollama-cloud-usage":              {},
	"PUT /api/v1/admin/accounts/:id/ollama-cloud-usage/auto-refresh": {},
	"POST /api/v1/admin/accounts/:id/ollama-cloud-usage/refresh":     {},
	"PUT /api/v1/admin/accounts/:id/ollama-cloud-usage/session":      {},
	"DELETE /api/v1/admin/accounts/:id/ollama-cloud-usage/session":   {},
	"POST /api/v1/admin/accounts/:id/recover-state":                  {},
	"POST /api/v1/admin/accounts/:id/refresh":                        {},
	"POST /api/v1/admin/accounts/:id/refresh-tier":                   {},
	"POST /api/v1/admin/accounts/:id/reset-quota":                    {},
	"POST /api/v1/admin/accounts/:id/revert-proxy-fallback":          {},
	"POST /api/v1/admin/accounts/:id/schedulable":                    {},
	"GET /api/v1/admin/accounts/:id/scheduled-test-plans":            {},
	"POST /api/v1/admin/accounts/:id/set-privacy":                    {},
	"POST /api/v1/admin/accounts/:id/shadow":                         {},
	"DELETE /api/v1/admin/accounts/:id/temp-unschedulable":           {},
	"POST /api/v1/admin/accounts/:id/test":                           {},
	"POST /api/v1/admin/accounts/:id/upstream-billing-probe":         {},
	"PUT /api/v1/admin/accounts/:id/upstream-billing-probe":          {},
	"GET /api/v1/admin/accounts/antigravity/default-model-mapping":   {},
	"POST /api/v1/admin/accounts/batch":                              {},
	"POST /api/v1/admin/accounts/batch-clear-error":                  {},
	"POST /api/v1/admin/accounts/batch-refresh":                      {},
	"POST /api/v1/admin/accounts/batch-refresh-tier":                 {},
	"POST /api/v1/admin/accounts/batch-update-credentials":           {},
	"POST /api/v1/admin/accounts/bulk-update":                        {},
	"POST /api/v1/admin/accounts/check-mixed-channel":                {},
	"POST /api/v1/admin/accounts/cookie-auth":                        {},
	"GET /api/v1/admin/accounts/data":                                {},
	"POST /api/v1/admin/accounts/data":                               {},
	"POST /api/v1/admin/accounts/exchange-code":                      {},
	"POST /api/v1/admin/accounts/exchange-setup-token-code":          {},
	"POST /api/v1/admin/accounts/generate-auth-url":                  {},
	"POST /api/v1/admin/accounts/generate-setup-token-url":           {},
	"POST /api/v1/admin/accounts/import/codex-session":               {},
	"POST /api/v1/admin/accounts/models/sync-upstream-preview":       {},
	"GET /api/v1/admin/accounts/ollama-cloud-usage/settings":         {},
	"PUT /api/v1/admin/accounts/ollama-cloud-usage/settings":         {},
	"POST /api/v1/admin/accounts/setup-token-cookie-auth":            {},
	"POST /api/v1/admin/accounts/sync/crs":                           {},
	"POST /api/v1/admin/accounts/sync/crs/preview":                   {},
	"POST /api/v1/admin/accounts/upstream-billing-probe/batch":       {},
	"GET /api/v1/admin/accounts/upstream-billing-probe/settings":     {},
	"PUT /api/v1/admin/accounts/upstream-billing-probe/settings":     {},

	// ---- groups（15 条）----
	// 分组：写操作 + 客户 API Key、订阅等客户数据
	"POST /api/v1/admin/groups":                                  {},
	"PUT /api/v1/admin/groups/:id":                               {},
	"DELETE /api/v1/admin/groups/:id":                            {},
	"GET /api/v1/admin/groups/:id/api-keys":                      {},
	"POST /api/v1/admin/groups/:id/composite-routes":             {},
	"PUT /api/v1/admin/groups/:id/composite-routes/:route_id":    {},
	"DELETE /api/v1/admin/groups/:id/composite-routes/:route_id": {},
	"POST /api/v1/admin/groups/:id/composite-routes/preview":     {},
	"POST /api/v1/admin/groups/:id/duplicate":                    {},
	"PUT /api/v1/admin/groups/:id/rate-multipliers":              {},
	"DELETE /api/v1/admin/groups/:id/rate-multipliers":           {},
	"PUT /api/v1/admin/groups/:id/rpm-overrides":                 {},
	"DELETE /api/v1/admin/groups/:id/rpm-overrides":              {},
	"GET /api/v1/admin/groups/:id/subscriptions":                 {},
	"PUT /api/v1/admin/groups/sort-order":                        {},

	// ---- channels（4 条）----
	// 渠道与定价：写操作 + 伪装成 GET 的上游同步
	"POST /api/v1/admin/channels":                    {},
	"PUT /api/v1/admin/channels/:id":                 {},
	"DELETE /api/v1/admin/channels/:id":              {},
	"GET /api/v1/admin/channels/pricing/sync-models": {},

	// ---- users（18 条）----
	// 用户：写操作 + 其他客户的密钥/财务/用量/配额
	"POST /api/v1/admin/users":                           {},
	"PUT /api/v1/admin/users/:id":                        {},
	"DELETE /api/v1/admin/users/:id":                     {},
	"GET /api/v1/admin/users/:id/api-keys":               {},
	"GET /api/v1/admin/users/:id/attributes":             {},
	"PUT /api/v1/admin/users/:id/attributes":             {},
	"POST /api/v1/admin/users/:id/auth-identities":       {},
	"POST /api/v1/admin/users/:id/balance":               {},
	"GET /api/v1/admin/users/:id/balance-history":        {},
	"GET /api/v1/admin/users/:id/platform-quotas":        {},
	"PUT /api/v1/admin/users/:id/platform-quotas":        {},
	"POST /api/v1/admin/users/:id/platform-quotas/reset": {},
	"POST /api/v1/admin/users/:id/replace-group":         {},
	"GET /api/v1/admin/users/:id/rpm-status":             {},
	"GET /api/v1/admin/users/:id/subscriptions":          {},
	"GET /api/v1/admin/users/:id/usage":                  {},
	"POST /api/v1/admin/users/batch-concurrency":         {},
	"POST /api/v1/admin/users/batch-limits":              {},

	// ---- ops（16 条）----
	// 运维：全部写端点（读端点见白名单）
	"PUT /api/v1/admin/ops/advanced-settings":           {},
	"PUT /api/v1/admin/ops/alert-events/:id/status":     {},
	"POST /api/v1/admin/ops/alert-rules":                {},
	"PUT /api/v1/admin/ops/alert-rules/:id":             {},
	"DELETE /api/v1/admin/ops/alert-rules/:id":          {},
	"POST /api/v1/admin/ops/alert-silences":             {},
	"PUT /api/v1/admin/ops/email-notification/config":   {},
	"PUT /api/v1/admin/ops/errors/:id/resolve":          {},
	"PUT /api/v1/admin/ops/performance-settings":        {},
	"PUT /api/v1/admin/ops/request-errors/:id/resolve":  {},
	"PUT /api/v1/admin/ops/runtime/alert":               {},
	"PUT /api/v1/admin/ops/runtime/logging":             {},
	"POST /api/v1/admin/ops/runtime/logging/reset":      {},
	"PUT /api/v1/admin/ops/settings/metric-thresholds":  {},
	"POST /api/v1/admin/ops/system-logs/cleanup":        {},
	"PUT /api/v1/admin/ops/upstream-errors/:id/resolve": {},
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
		key := r.Method + " " + r.Path
		if _, ok := allow[key]; ok {
			continue
		}
		if _, ok := reviewedDenyExact[key]; ok {
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
			unclassified = append(unclassified, key)
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

// TestReadonlyAdminDenyPrefixesDoNotCoverPartialModules 守住"精确 / 前缀"两套机制的边界。
//
// 背景：拒绝侧按【路径前缀】匹配，且不区分方法。一旦有人把 "/api/v1/admin/accounts"
// 这类模块根塞进 reviewedDenyPrefixes（很有诱惑力 —— 集合级写端点 POST
// /api/v1/admin/accounts 的完整路径就是模块根，加一条前缀就能"清账"），该模块内
// 所有未登记路由都会被自动判为"已审阅拒绝"，完备性测试从此不再报告它们。
//
// 运行时依然安全（未进白名单的路由一律 403），但强制决策的机制在【最需要它的五个
// 模块】上失效了 —— 这五个模块正是 readonly_admin 唯一能触达的界面，其中新增端点
// 到底该放行还是拒绝，必须有人明确拍板，而不是悄悄默认成 403。
//
// 因此：这五个模块内部的拒绝项一律登记到 reviewedDenyExact（精确匹配、逐条枚举）。
func TestReadonlyAdminDenyPrefixesDoNotCoverPartialModules(t *testing.T) {
	for _, prefix := range reviewedDenyPrefixes {
		for _, root := range partiallyAllowedModuleRoots {
			overlaps := strings.HasPrefix(root, prefix) || strings.HasPrefix(prefix, root)
			require.Falsef(t, overlaps,
				"reviewedDenyPrefixes 中的 %q 覆盖了部分放行模块 %q。\n"+
					"这五个模块既有放行端点也有拒绝端点，用前缀登记会让模块内新增的路由\n"+
					"自动被判为『已审阅拒绝』，完备性测试从此不再要求任何人做决策 ——\n"+
					"而这正是本套测试存在的唯一理由。\n"+
					"请把该模块内需要拒绝的路由逐条加入 reviewedDenyExact（精确 METHOD+路径），\n"+
					"而不是删掉这条断言。", prefix, root)
		}
	}
}

// TestReadonlyAdminDenyExactStaysInsidePartialModules 反向约束：reviewedDenyExact
// 只服务于那五个部分放行的模块。整体拒绝的模块请用 reviewedDenyPrefixes，逐条枚举
// 它们没有意义，只会让本表淹没在噪音里，掩盖真正需要人工盯防的五个模块。
func TestReadonlyAdminDenyExactStaysInsidePartialModules(t *testing.T) {
	for key := range reviewedDenyExact {
		parts := strings.SplitN(key, " ", 2)
		require.Lenf(t, parts, 2, "reviewedDenyExact 条目格式应为 \"METHOD /path\"，实际为 %q", key)
		path := parts[1]
		inside := false
		for _, root := range partiallyAllowedModuleRoots {
			if path == root || strings.HasPrefix(path, root+"/") {
				inside = true
				break
			}
		}
		require.Truef(t, inside,
			"reviewedDenyExact 中的 %q 不属于任何部分放行模块 %v。\n"+
				"整体拒绝的模块请登记到 reviewedDenyPrefixes。", key, partiallyAllowedModuleRoots)
	}
}
