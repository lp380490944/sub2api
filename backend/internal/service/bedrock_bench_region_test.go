package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type benchRegionRepoStub struct {
	AccountRepository
	rateLimitedUntil map[int64]time.Time
	tempUnschedUntil map[int64]time.Time
}

func (r *benchRegionRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedUntil[id] = resetAt
	return nil
}
func (r *benchRegionRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, _ string) error {
	r.tempUnschedUntil[id] = until
	return nil
}

type fakeBedrockFailCounter struct{ n int64 }

func (f *fakeBedrockFailCounter) IncrementBedrockFailureCount(_ context.Context, _ int64, _ int) (int64, error) {
	f.n++
	return f.n, nil
}
func (f *fakeBedrockFailCounter) ResetBedrockFailureCount(_ context.Context, _ int64) error {
	f.n = 0
	return nil
}

func newBenchRegionService(counter BedrockFailureCounterCache) (*RateLimitService, *benchRegionRepoStub) {
	repo := &benchRegionRepoStub{
		rateLimitedUntil: map[int64]time.Time{},
		tempUnschedUntil: map[int64]time.Time{},
	}
	cfg := &config.Config{}
	cfg.Gateway.BedrockRPMCooldownSec = 30
	cfg.Gateway.BedrockDailyQuotaCooldownEnabled = true
	cfg.Gateway.BedrockFailureThreshold = 2
	cfg.Gateway.BedrockFailureWindowSec = 300
	cfg.Gateway.BedrockFailureCooldownSec = 600
	svc := &RateLimitService{accountRepo: repo, cfg: cfg}
	if counter != nil {
		svc.SetBedrockFailureCounterCache(counter)
	}
	return svc, repo
}

func bedrockPoolAccount() *Account {
	return &Account{
		ID:       120,
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
		Credentials: map[string]any{
			"auth_mode": "apikey",
			"pool_mode": true, // 关键:池模式下仍须冷却
		},
	}
}

func TestBenchBedrockRegion_PoolModeStillRateLimited(t *testing.T) {
	svc, repo := newBenchRegionService(&fakeBedrockFailCounter{})
	body := []byte(`{"message":"Too many requests, please wait before trying again."}`)
	svc.BenchBedrockRegion(context.Background(), bedrockPoolAccount(), http.Header{}, body)
	if _, ok := repo.rateLimitedUntil[120]; !ok {
		t.Fatal("pool_mode Bedrock 未被 SetRateLimited(冷却缺口未修复)")
	}
}

func TestBenchBedrockRegion_ConsecutiveFailureTempUnsched(t *testing.T) {
	svc, repo := newBenchRegionService(&fakeBedrockFailCounter{})
	body := []byte(`{"message":"Too many requests"}`)
	acc := bedrockPoolAccount()
	svc.BenchBedrockRegion(context.Background(), acc, http.Header{}, body) // n=1
	if _, ok := repo.tempUnschedUntil[120]; ok {
		t.Fatal("首次失败不应临时封禁")
	}
	svc.BenchBedrockRegion(context.Background(), acc, http.Header{}, body) // n=2 → 达阈值
	if _, ok := repo.tempUnschedUntil[120]; !ok {
		t.Fatal("连续 2 次失败应触发 SetTempUnschedulable")
	}
}

func TestBenchBedrockRegion_NonBedrockIgnored(t *testing.T) {
	svc, repo := newBenchRegionService(&fakeBedrockFailCounter{})
	nonBedrock := &Account{ID: 5, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	svc.BenchBedrockRegion(context.Background(), nonBedrock, http.Header{}, []byte(`{}`))
	if len(repo.rateLimitedUntil) != 0 {
		t.Fatal("非 Bedrock 账号不应被冷却")
	}
}

// handle429 不再持有 Bedrock 分支：Bedrock apikey 账号的冷却完全由
// BenchBedrockRegion(gateway_bedrock.go 的 failover hook)独占写入。
// 非池模式的 Bedrock 429 若仍落到 handle429 的 Anthropic 兜底分支(因 platform
// 是 PlatformAnthropic),会产生一次冗余的 5s 兜底冷却，随后被 BenchBedrockRegion
// 的语义化冷却覆盖——本测试确认 handle429 对 Bedrock apikey 账号提前返回，
// 完全不触碰 accountRepo（即 apply429FallbackRateLimit 未被调用）。
func TestHandle429_BedrockAPIKeySkipsLocalFallback(t *testing.T) {
	svc, repo := newBenchRegionService(nil)
	// 非池模式：不设置 pool_mode，模拟直连/非池 Bedrock apikey 账号。
	account := &Account{
		ID:          121,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeBedrock,
		Credentials: map[string]any{"auth_mode": "apikey"},
	}
	// 空 headers + 无 Anthropic 限流头，模拟真实 Bedrock 429（不携带
	// anthropic-ratelimit-unified-* 头）。若 handle429 未提前返回，会走到
	// account.Platform == PlatformAnthropic 的兜底分支并调用 apply429FallbackRateLimit。
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"message":"Too many requests"}`))
	if len(repo.rateLimitedUntil) != 0 {
		t.Fatal("handle429 不应为 Bedrock apikey 账号写入冷却(应由 BenchBedrockRegion 独占写入,handle429 需提前 return)")
	}
	if len(repo.tempUnschedUntil) != 0 {
		t.Fatal("handle429 不应为 Bedrock apikey 账号写入临时不可调度状态")
	}
}
