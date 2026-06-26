package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDetectInterceptType_MaxTokensOneHaikuRequiresClaudeCodeClient(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	notClaudeCode := detectInterceptType(body, "claude-haiku-4-5", 1, false, false)
	require.Equal(t, InterceptTypeNone, notClaudeCode)

	isClaudeCode := detectInterceptType(body, "claude-haiku-4-5", 1, false, true)
	require.Equal(t, InterceptTypeMaxTokensOneHaiku, isClaudeCode)

	// fork: 流式 max_tokens=1 haiku 探测不拦截（mock 应答是非流式 JSON）
	streaming := detectInterceptType(body, "claude-haiku-4-5", 1, true, true)
	require.Equal(t, InterceptTypeNone, streaming)
}

func TestDetectInterceptType_SuggestionModeUnaffected(t *testing.T) {
	body := []byte(`{
		"messages":[{
			"role":"user",
			"content":[{"type":"text","text":"[SUGGESTION MODE:foo]"}]
		}],
		"system":[]
	}`)

	got := detectInterceptType(body, "claude-sonnet-4-5", 256, false, false)
	require.Equal(t, InterceptTypeSuggestionMode, got)
}

func TestSendMockInterceptResponse_MaxTokensOneHaiku(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	sendMockInterceptResponse(ctx, "claude-haiku-4-5", InterceptTypeMaxTokensOneHaiku)

	require.Equal(t, http.StatusOK, rec.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "max_tokens", response["stop_reason"])

	id, ok := response["id"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(id, "msg_bdrk_"))

	content, ok := response["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)

	firstBlock, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "#", firstBlock["text"])

	usage, ok := response["usage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), usage["output_tokens"])
}

func TestClaudeProbeDedupCache_DeduplicatesWithinTTL(t *testing.T) {
	now := time.Date(2026, 5, 2, 5, 0, 0, 0, time.UTC)
	cache := newClaudeProbeDedupCache(time.Minute)
	cache.now = func() time.Time { return now }

	body := []byte(`{"model":"claude-haiku-4-5","max_tokens":1}`)

	require.False(t, cache.SeenOrStore(11, 22, "claude-haiku-4-5", body))
	require.True(t, cache.SeenOrStore(11, 22, "claude-haiku-4-5", body))

	require.False(t, cache.SeenOrStore(12, 22, "claude-haiku-4-5", body), "api key must isolate probe cache")
	require.False(t, cache.SeenOrStore(11, 23, "claude-haiku-4-5", body), "group must isolate probe cache")
	require.False(t, cache.SeenOrStore(11, 22, "claude-haiku-4-5-20251001", body), "model must isolate probe cache")
	require.False(t, cache.SeenOrStore(11, 22, "claude-haiku-4-5", []byte(`{"model":"claude-haiku-4-5","max_tokens":1,"metadata":{"user_id":"other"}}`)), "body hash must isolate probe cache")

	now = now.Add(61 * time.Second)
	require.False(t, cache.SeenOrStore(11, 22, "claude-haiku-4-5", body), "expired probes should be treated as new")
}
