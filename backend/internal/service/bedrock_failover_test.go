package service

import (
	"testing"

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
