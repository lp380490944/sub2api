package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const bedrockFailureCounterPrefix = "bedrock_fail_count:account:"

var bedrockFailureCounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])
	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end
	return count
`)

type bedrockFailureCounterCache struct {
	rdb *redis.Client
}

// NewBedrockFailureCounterCache 创建 Bedrock 连续失败计数器缓存实例。
func NewBedrockFailureCounterCache(rdb *redis.Client) service.BedrockFailureCounterCache {
	return &bedrockFailureCounterCache{rdb: rdb}
}

func (c *bedrockFailureCounterCache) IncrementBedrockFailureCount(ctx context.Context, accountID int64, windowSec int) (int64, error) {
	if windowSec <= 0 {
		windowSec = 300
	}
	key := fmt.Sprintf("%s%d", bedrockFailureCounterPrefix, accountID)
	result, err := bedrockFailureCounterIncrScript.Run(ctx, c.rdb, []string{key}, windowSec).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment bedrock failure count: %w", err)
	}
	return result, nil
}

func (c *bedrockFailureCounterCache) ResetBedrockFailureCount(ctx context.Context, accountID int64) error {
	key := fmt.Sprintf("%s%d", bedrockFailureCounterPrefix, accountID)
	return c.rdb.Del(ctx, key).Err()
}
