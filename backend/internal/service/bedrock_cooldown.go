package service

import (
	"net/http"
	"strings"
	"time"
)

type bedrock429Kind int

const (
	bedrock429RPMThrottle bedrock429Kind = iota // 短时 RPM 限流(保守默认)
	bedrock429DailyQuota                        // 当天日配额耗尽
)

// classifyBedrock429 根据 429 响应 body 区分"日配额耗尽"与"短时 RPM 限流"。
// 无法识别时保守归为 RPM(短冷却),交由连续失败计数器兜底。
func classifyBedrock429(body []byte) bedrock429Kind {
	msg := strings.ToLower(extractUpstreamErrorMessage(body))
	if msg == "" {
		msg = strings.ToLower(string(body))
	}
	if strings.Contains(msg, "per day") || strings.Contains(msg, "tokens per day") {
		return bedrock429DailyQuota
	}
	return bedrock429RPMThrottle
}

// nextUTCMidnight 返回 now 之后的下一个 UTC 00:00。
func nextUTCMidnight(now time.Time) time.Time {
	u := now.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
}

// bedrockCooldownUntil 计算区域账号应冷却到的绝对时间。
//   - daily_quota 且开关开启 → 次日 UTC 0 点(自然 ≤24h)
//   - 否则 → Retry-After(若有) else rpmDefaultSec,clamp [1,3600]s
func bedrockCooldownUntil(now time.Time, kind bedrock429Kind, headers http.Header, rpmDefaultSec int, dailyEnabled bool) time.Time {
	if kind == bedrock429DailyQuota && dailyEnabled {
		return nextUTCMidnight(now)
	}
	sec := rpmDefaultSec
	if sec <= 0 {
		sec = 30
	}
	if ra := parseRetryAfterSeconds(headers.Get("Retry-After")); ra > 0 {
		sec = ra
	}
	if sec < 1 {
		sec = 1
	}
	if sec > 3600 {
		sec = 3600
	}
	return now.Add(time.Duration(sec) * time.Second)
}
