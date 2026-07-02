package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSoftenBedrockExhaustStatus(t *testing.T) {
	// Bedrock soft remap: 429 -> 529 (so mapUpstreamError yields 503 overloaded_error)
	assert.Equal(t, 529, softenBedrockExhaustStatus(http.StatusTooManyRequests, true))
	// not soft: unchanged
	assert.Equal(t, http.StatusTooManyRequests, softenBedrockExhaustStatus(http.StatusTooManyRequests, false))
	// soft but not 429: unchanged
	assert.Equal(t, http.StatusBadGateway, softenBedrockExhaustStatus(http.StatusBadGateway, true))
}
