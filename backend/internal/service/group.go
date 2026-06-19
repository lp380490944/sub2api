package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig

type Group struct {
	ID             int64
	Name           string
	Description    string
	Platform       string
	RateMultiplier float64
	IsExclusive    bool
	Status         string
	Hydrated       bool // indicates the group was loaded from a trusted repository source

	SubscriptionType    string
	DailyLimitUSD       *float64
	WeeklyLimitUSD      *float64
	MonthlyLimitUSD     *float64
	DefaultValidityDays int

	// 图片生成计费配置（antigravity 和 gemini 平台使用）
	AllowImageGeneration bool
	ImageRateIndependent bool
	ImageRateMultiplier  float64
	ImagePrice1K         *float64
	ImagePrice2K         *float64
	ImagePrice4K         *float64

	// Claude Code 客户端限制
	ClaudeCodeOnly  bool
	FallbackGroupID *int64
	// 无效请求兜底分组（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64

	// 模型路由配置
	// key: 模型匹配模式（支持 * 通配符，如 "claude-opus-*"）
	// value: 优先账号 ID 列表
	ModelRouting        map[string][]int64
	ModelRoutingEnabled bool

	// DefaultModelRateLimits 分组级按模型 USD 配额默认规则；API Key 未配置
	// ModelRateLimits 时继承此值。
	DefaultModelRateLimits ModelRateLimits

	// MCP XML 协议注入开关（仅 antigravity 平台使用）
	MCPXMLInject bool

	// 支持的模型系列（仅 antigravity 平台使用）
	// 可选值: claude, gemini_text, gemini_image
	SupportedModelScopes []string

	// 分组排序
	SortOrder int

	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool
	RequireOAuthOnly            bool // 仅允许非 apikey 类型账号关联（OpenAI/Antigravity/Anthropic/Gemini）
	RequirePrivacySet           bool // 调度时仅允许 privacy 已成功设置的账号（OpenAI/Antigravity/Anthropic/Gemini）
	DefaultMappedModel          string
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	ModelsListConfig            GroupModelsListConfig

	// RPMLimit 分组级每分钟请求数上限（0 = 不限制）。
	// 一旦设置即接管该分组用户的限流（覆盖用户级 rpm_limit），可被 user-group rpm_override 进一步覆盖。
	RPMLimit int

	// ── M2：分组级策略默认值（三级覆盖链：account > group > system） ──

	// DefaultAccountConcurrency 是该分组内账号的并发默认值（0 = 继承系统全局）。
	DefaultAccountConcurrency int
	// DefaultAccountRPM 是该分组内账号的 RPM 默认值（0 = 继承系统全局）。
	// 与 RPMLimit 不同：RPMLimit 是整个分组的池级上限，DefaultAccountRPM 是各账号单独的默认值。
	DefaultAccountRPM int
	// DefaultPassthroughProfile 是该分组的透传 profile（"" = 继承系统；transparent/protected/strict）。
	DefaultPassthroughProfile string
	// Default429CooldownSec 是该分组的 429 冷却秒数（0 = 继承系统全局）。
	Default429CooldownSec int

	CreatedAt time.Time
	UpdatedAt time.Time

	AccountGroups           []AccountGroup
	AccountCount            int64
	ActiveAccountCount      int64
	RateLimitedAccountCount int64
}

func (g *Group) IsActive() bool {
	return g.Status == StatusActive
}

func (g *Group) IsSubscriptionType() bool {
	return g.SubscriptionType == SubscriptionTypeSubscription
}

func (g *Group) HasDailyLimit() bool {
	return g.DailyLimitUSD != nil && *g.DailyLimitUSD > 0
}

func (g *Group) HasWeeklyLimit() bool {
	return g.WeeklyLimitUSD != nil && *g.WeeklyLimitUSD > 0
}

func (g *Group) HasMonthlyLimit() bool {
	return g.MonthlyLimitUSD != nil && *g.MonthlyLimitUSD > 0
}

// GetImagePrice 根据 image_size 返回对应的图片生成价格
// 如果分组未配置价格，返回 nil（调用方应使用默认值）
func (g *Group) GetImagePrice(imageSize string) *float64 {
	switch imageSize {
	case "1K":
		return g.ImagePrice1K
	case "2K":
		return g.ImagePrice2K
	case "4K":
		return g.ImagePrice4K
	default:
		// 未知尺寸默认按 2K 计费
		return g.ImagePrice2K
	}
}

// IsGroupContextValid reports whether a group from context has the fields required for routing decisions.
func IsGroupContextValid(group *Group) bool {
	if group == nil {
		return false
	}
	if group.ID <= 0 {
		return false
	}
	if !group.Hydrated {
		return false
	}
	if group.Platform == "" || group.Status == "" {
		return false
	}
	return true
}

// GetRoutingAccountIDs 根据请求模型获取路由账号 ID 列表
// 返回匹配的优先账号 ID 列表（含 "*" 兜底梯队，已按梯队顺序去重）；
// 如果没有匹配规则则返回 nil。
func (g *Group) GetRoutingAccountIDs(requestedModel string) []int64 {
	ids, _ := g.GetRoutingAccountIDsTiered(requestedModel)
	return ids
}

// GetRoutingAccountIDsTiered 返回模型路由的分层优先账号列表。
//
// 返回值：
//   - ids: 按优先梯队排序的账号 ID（命中的具体规则在前，"*" 通配兜底在后），已去重；
//   - tierRank: accountID -> 梯队序号（0 = 最高优先梯队，数字越小越优先）。
//
// 设计意图：用户在模型路由里既配置了具体模型规则（如 claude-opus-4-7 → 中转账号），
// 又配置了 "*" 兜底规则（如 * → AWS 账号）。调度时应**先穷尽具体规则的优先账号，
// 全部不可用后才回落到 "*" 账号**。把 "*" 作为一个严格更低的梯队追加进来，让选号逻辑
// 用 tierRank 作为第一排序关键字即可保证该语义。
//
// 路由未启用或无匹配时返回 (nil, nil)。
func (g *Group) GetRoutingAccountIDsTiered(requestedModel string) ([]int64, map[int64]int) {
	if !g.ModelRoutingEnabled || len(g.ModelRouting) == 0 || requestedModel == "" {
		return nil, nil
	}

	matchedPattern, matchedIDs := g.matchRoutingRule(requestedModel)
	if len(matchedIDs) == 0 {
		return nil, nil
	}

	ordered := make([]int64, 0, len(matchedIDs)+4)
	tierRank := make(map[int64]int, len(matchedIDs)+4)
	appendTier := func(ids []int64, tier int) {
		for _, id := range ids {
			if id <= 0 {
				continue
			}
			if _, dup := tierRank[id]; dup {
				continue
			}
			tierRank[id] = tier
			ordered = append(ordered, id)
		}
	}

	// 梯队 0：命中的具体规则
	appendTier(matchedIDs, 0)
	// 梯队 1："*" 兜底规则（仅当本次命中的规则本身不是 "*" 时追加）
	if matchedPattern != "*" {
		if wildcardIDs, ok := g.ModelRouting["*"]; ok && len(wildcardIDs) > 0 {
			appendTier(wildcardIDs, 1)
		}
	}

	return ordered, tierRank
}

// matchRoutingRule 返回命中请求模型的最具体路由规则及其账号列表。
// 优先级：精确匹配 > 最长前缀通配匹配（"*" 前缀为空，天然最后兜底）。
// 无匹配时返回 ("", nil)。
func (g *Group) matchRoutingRule(requestedModel string) (string, []int64) {
	// 1. 精确匹配优先
	if accountIDs, ok := g.ModelRouting[requestedModel]; ok && len(accountIDs) > 0 {
		return requestedModel, accountIDs
	}

	// 2. 通配符匹配：选最长前缀（最具体）的规则，确保确定性且让 "*" 兜底
	bestPattern := ""
	bestPrefixLen := -1
	for pattern, accountIDs := range g.ModelRouting {
		if len(accountIDs) == 0 || !matchModelPattern(pattern, requestedModel) {
			continue
		}
		prefixLen := len(strings.TrimSuffix(pattern, "*"))
		if prefixLen > bestPrefixLen {
			bestPrefixLen = prefixLen
			bestPattern = pattern
		}
	}
	if bestPattern != "" {
		return bestPattern, g.ModelRouting[bestPattern]
	}

	return "", nil
}

// matchModelPattern 检查模型是否匹配模式
// 支持 * 通配符，如 "claude-opus-*" 匹配 "claude-opus-4-20250514"
func matchModelPattern(pattern, model string) bool {
	if pattern == model {
		return true
	}

	// 处理 * 通配符（仅支持末尾通配符）
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	return false
}
