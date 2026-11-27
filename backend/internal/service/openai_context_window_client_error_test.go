package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testContextWindowMessage = "Your input exceeds the context window of this model. Please adjust your input and try again."

// 上下文超限挂在 HTTP 200 SSE 的 response.failed / error 帧上时，语义状态必须是
// 400：502 会让下游网关把确定性请求失败当成上游故障跨渠道重放。
func TestOpenAIStreamFailureStatus_ContextWindowIs400(t *testing.T) {
	for name, payload := range map[string]string{
		"response_failed": `{"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"` + testContextWindowMessage + `"}}}`,
		"bare_error":      `{"type":"error","error":{"type":"upstream_error","message":"` + testContextWindowMessage + `"}}`,
		"error_code":      `{"type":"response.failed","response":{"error":{"code":"context_length_exceeded","message":"too many tokens"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			message := extractOpenAISSEErrorMessage([]byte(payload))
			require.Equal(t, http.StatusBadRequest, openAIStreamFailureStatus([]byte(payload), message))
			require.False(t, openAIStreamFailedEventShouldFailover([]byte(payload), message))
		})
	}

	// 其它不可识别的上游失败仍是 502。
	other := []byte(`{"type":"response.failed","response":{"error":{"code":"upstream_error","message":"temporary upstream outage"}}}`)
	require.Equal(t, http.StatusBadGateway, openAIStreamFailureStatus(other, "temporary upstream outage"))
}

func TestOpenAIContextWindowClientStatus(t *testing.T) {
	status, errType, ok := openAIContextWindowClientStatus(testContextWindowMessage, nil)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "invalid_request_error", errType)

	_, _, ok = openAIContextWindowClientStatus("temporary upstream outage", []byte(`{"error":{"message":"temporary upstream outage"}}`))
	require.False(t, ok)
}

// 兼容路径（Chat Completions / Anthropic Messages）上游以 5xx 包装上下文超限时，
// 同样回 400 invalid_request_error，并保留上游文案。
func TestHandleCompatErrorResponse_ContextWindow5xxReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	svc := &OpenAIGatewayService{}
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"` + testContextWindowMessage + `","type":"upstream_error"}}`)),
		Header:     http.Header{},
	}
	account := &Account{ID: 14, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := svc.handleCompatErrorResponse(resp, c, account, writeChatCompletionsError)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid_request_error")
	require.Contains(t, rec.Body.String(), "exceeds the context window")
}

// 账号启用了自定义错误码且上游状态不在列表内时，普通错误回通用 500；上下文超限
// 是请求级错误，仍必须回 400，否则下游网关照样把它当上游故障重放。
func TestContextWindow_CustomErrorCodesStillReturn400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newAccount := func() *Account {
		return &Account{
			ID:       14,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(429)},
			},
		}
	}
	newResp := func(message string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"` + message + `","type":"upstream_error"}}`)),
			Header:     http.Header{},
		}
	}
	require.False(t, newAccount().ShouldHandleErrorCode(http.StatusBadGateway))

	t.Run("responses", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		_, err := (&OpenAIGatewayService{}).handleErrorResponse(c.Request.Context(), newResp(testContextWindowMessage), c, newAccount(), nil)
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "context_length_exceeded")
		require.Contains(t, rec.Body.String(), "exceeds the context window")
	})

	t.Run("compat", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		_, err := (&OpenAIGatewayService{}).handleCompatErrorResponse(newResp(testContextWindowMessage), c, newAccount(), writeAnthropicError)
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "invalid_request_error")
		require.Contains(t, rec.Body.String(), "exceeds the context window")
	})

	t.Run("other_errors_stay_generic_500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		_, err := (&OpenAIGatewayService{}).handleErrorResponse(c.Request.Context(), newResp("temporary upstream outage"), c, newAccount(), nil)
		require.Error(t, err)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.Contains(t, rec.Body.String(), "Upstream gateway error")
	})
}
