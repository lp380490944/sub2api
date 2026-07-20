package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Regression coverage for the Bedrock (SigV4/API-key) and Anthropic API-key
// passthrough forward paths: a transport-level upstream failure (DoWithTLS
// returning err, not an HTTP response — e.g. "dial tcp: no route to host" on
// an unreachable region endpoint) must return *UpstreamFailoverError and
// temp-unschedule the account, exactly like the main Forward() path does
// since dc0bf76b7. Before this fix both paths still wrote a plain 502 to the
// client and returned a plain error, so the dead region account was never
// benched and sticky sessions kept routing every request back to it.

func newBedrockTransportErrTestAccount() *Account {
	return &Account{
		ID:          9002,
		Name:        "bedrock-me-south-1-global",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeBedrock,
		Concurrency: 1,
		Credentials: map[string]any{
			"auth_mode": "apikey",
			"api_key":   "test-bedrock-api-key",
			"region":    "me-south-1",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func newBedrockTransportErrService(upstream *transportErrHTTPUpstream, repo *transportErrAccountRepoStub) *GatewayService {
	return &GatewayService{
		cfg:              &config.Config{},
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: &RateLimitService{accountRepo: repo},
	}
}

func TestBedrockTransportError_ReturnsFailoverErrorForAccountSwitch(t *testing.T) {
	account := newBedrockTransportErrTestAccount()
	repo := &transportErrAccountRepoStub{account: account}
	upstream := &transportErrHTTPUpstream{
		err: errors.New(`Post "https://bedrock-runtime.me-south-1.amazonaws.com/model/global.anthropic.claude-opus-4-6-v1/invoke-with-response-stream": dial tcp 10.20.30.40:443: connect: no route to host`),
	}
	svc := newBedrockTransportErrService(upstream, repo)

	c, rec := newTransportErrTestContext()
	resp, err := svc.executeBedrockUpstream(context.Background(), c, account,
		[]byte(`{}`), "global.anthropic.claude-opus-4-6-v1", "me-south-1", true, nil, "test-bedrock-api-key", "")
	require.Nil(t, resp)

	var fo *UpstreamFailoverError
	require.True(t, errors.As(err, &fo), "bedrock transport-level error must return *UpstreamFailoverError so the handler can fail over to another region account")
	require.False(t, fo.RetryableOnSameAccount)
	require.Equal(t, http.StatusBadGateway, fo.StatusCode)
	require.Equal(t, 0, rec.Body.Len(), "must not write a hard response on a failover-triggering transport error")
	require.Equal(t, 1, upstream.calls)
}

func TestBedrockTransportError_TempUnschedulesAccount(t *testing.T) {
	account := newBedrockTransportErrTestAccount()
	repo := &transportErrAccountRepoStub{account: account}
	upstream := &transportErrHTTPUpstream{err: errors.New(`dial tcp: connect: connection refused`)}
	svc := newBedrockTransportErrService(upstream, repo)

	c, _ := newTransportErrTestContext()
	before := time.Now()
	_, err := svc.executeBedrockUpstream(context.Background(), c, account,
		[]byte(`{}`), "global.anthropic.claude-opus-4-6-v1", "me-south-1", true, nil, "test-bedrock-api-key", "")
	require.Error(t, err)

	require.Len(t, repo.stepCalls, 1, "transport failure must temp-unschedule the bedrock account exactly once")
	call := repo.stepCalls[0]
	require.Equal(t, account.ID, call.accountID)
	require.True(t, call.until.After(before))
	require.Contains(t, call.reason, "connection refused")

	account.TempUnschedulableUntil = &call.until
	require.False(t, account.IsSchedulable(), "account must leave the schedulable pool after a transport failure")
}

func TestBedrockTransportError_ContextCanceled_NoFailoverNoEviction(t *testing.T) {
	account := newBedrockTransportErrTestAccount()
	repo := &transportErrAccountRepoStub{account: account}
	upstream := &transportErrHTTPUpstream{err: context.Canceled}
	svc := newBedrockTransportErrService(upstream, repo)

	c, rec := newTransportErrTestContext()
	_, err := svc.executeBedrockUpstream(context.Background(), c, account,
		[]byte(`{}`), "global.anthropic.claude-opus-4-6-v1", "me-south-1", true, nil, "test-bedrock-api-key", "")

	var fo *UpstreamFailoverError
	require.False(t, errors.As(err, &fo), "client disconnect must not trigger account failover")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, repo.stepCalls, "client disconnect must not temp-unschedule the account")
	require.Equal(t, 0, rec.Body.Len())
}

// The Anthropic API-key passthrough path (also used by Bedrock Mantle
// accounts) had the same stale transport-error branch.

func TestAnthropicPassthroughTransportError_ReturnsFailoverErrorAndBenches(t *testing.T) {
	account := newClaudePlatformAWSAccountForTest()
	repo := &transportErrAccountRepoStub{account: account}
	upstream := &transportErrHTTPUpstream{
		err: errors.New(`Post "https://aws-external-anthropic.ap-northeast-1.api.aws/v1/messages": dial tcp: connect: no route to host`),
	}
	svc := &GatewayService{
		cfg:              &config.Config{},
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: &RateLimitService{accountRepo: repo},
	}

	c, rec := newTransportErrTestContext()
	result, err := svc.Forward(context.Background(), c, account, newTransportErrTestParsed("claude-opus-4-6"))
	require.Nil(t, result)

	var fo *UpstreamFailoverError
	require.True(t, errors.As(err, &fo), "passthrough transport-level error must return *UpstreamFailoverError so the handler can fail over")
	require.Equal(t, http.StatusBadGateway, fo.StatusCode)
	require.Equal(t, 0, rec.Body.Len(), "must not write a hard response on a failover-triggering transport error")

	require.Len(t, repo.stepCalls, 1, "transport failure must temp-unschedule the passthrough account exactly once")
	require.Equal(t, account.ID, repo.stepCalls[0].accountID)
}

func TestAnthropicPassthroughTransportError_ContextCanceled_NoFailoverNoEviction(t *testing.T) {
	account := newClaudePlatformAWSAccountForTest()
	repo := &transportErrAccountRepoStub{account: account}
	upstream := &transportErrHTTPUpstream{err: context.Canceled}
	svc := &GatewayService{
		cfg:              &config.Config{},
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: &RateLimitService{accountRepo: repo},
	}

	c, rec := newTransportErrTestContext()
	_, err := svc.Forward(context.Background(), c, account, newTransportErrTestParsed("claude-opus-4-6"))

	var fo *UpstreamFailoverError
	require.False(t, errors.As(err, &fo), "client disconnect must not trigger account failover")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, repo.stepCalls, "client disconnect must not temp-unschedule the account")
	require.Equal(t, 0, rec.Body.Len())
}
