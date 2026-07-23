package service

import "context"

// BedrockFailureCounterCache 追踪 Bedrock 区域账号在滑动窗口内的连续 failover 失败次数。
type BedrockFailureCounterCache interface {
	// IncrementBedrockFailureCount 原子递增计数并返回当前值;首个计数时设置 windowSec TTL。
	IncrementBedrockFailureCount(ctx context.Context, accountID int64, windowSec int) (int64, error)
	// ResetBedrockFailureCount 清零计数器(成功响应时调用)。
	ResetBedrockFailureCount(ctx context.Context, accountID int64) error
}
