package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func bedrockAPIKeyAccount() *Account {
	return &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "apikey"}}
}

func TestBedrockShouldFastFailover(t *testing.T) {
	bedrock := bedrockAPIKeyAccount()
	oauth := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	// enabled + Bedrock apikey + retryable status -> true
	for _, code := range []int{429, 500, 502, 503, 504, 529} {
		assert.True(t, bedrockShouldFastFailover(bedrock, code, true), "code %d", code)
	}
	// non-retryable status -> false (genuine client errors pass through)
	for _, code := range []int{400, 401, 403, 404, 422} {
		assert.False(t, bedrockShouldFastFailover(bedrock, code, true), "code %d", code)
	}
	// disabled -> false even for retryable
	assert.False(t, bedrockShouldFastFailover(bedrock, 429, false))
	// non-Bedrock account -> false
	assert.False(t, bedrockShouldFastFailover(oauth, 429, true))
	// nil account -> false
	assert.False(t, bedrockShouldFastFailover(nil, 429, true))
}

func TestParseRetryAfterSeconds(t *testing.T) {
	assert.Equal(t, 0, parseRetryAfterSeconds(""))
	assert.Equal(t, 0, parseRetryAfterSeconds("  "))
	assert.Equal(t, 30, parseRetryAfterSeconds("30"))
	assert.Equal(t, 0, parseRetryAfterSeconds("-5"))
	assert.Equal(t, 0, parseRetryAfterSeconds("Wed, 21 Oct 2026 07:28:00 GMT")) // HTTP-date not honored
}

func TestBedrockThrottleCooldown(t *testing.T) {
	// default when no Retry-After
	assert.Equal(t, 10*time.Second, bedrockThrottleCooldown(http.Header{}, 10))
	// Retry-After overrides default
	h := http.Header{"Retry-After": {"30"}}
	assert.Equal(t, 30*time.Second, bedrockThrottleCooldown(h, 10))
	// clamp low
	assert.Equal(t, 1*time.Second, bedrockThrottleCooldown(http.Header{}, 0))
	// clamp high (Retry-After absurdly large)
	big := http.Header{"Retry-After": {"999999"}}
	assert.Equal(t, 3600*time.Second, bedrockThrottleCooldown(big, 10))
}
