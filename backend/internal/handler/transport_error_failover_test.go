package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// transportErrFailoverErr mirrors exactly the *UpstreamFailoverError that
// GatewayService.Forward now returns when the upstream HTTP client fails at
// the transport level (e.g. "dial tcp: no route to host") — see
// gateway_forward.go's DoWithTLS err != nil branch.
func transportErrFailoverErr() *service.UpstreamFailoverError {
	return &service.UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: []byte(`{"type":"error","error":{"type":"upstream_error","message":"Upstream request failed"}}`),
	}
}

// TestTransportError_AllAccountsFail_ClientGetsSingleTerminal502 simulates a
// group where every account is TCP-unreachable (the me-south-1 style bug):
// each account failure switches to the next one via HandleFailoverError, and
// once every account has failed the handler must write exactly one terminal
// 502 response — never a double-write or a concatenation of per-account
// failures.
func TestTransportError_AllAccountsFail_ClientGetsSingleTerminal502(t *testing.T) {
	mock := &mockTempUnscheduler{}
	const numAccounts = 3
	fs := NewFailoverState(numAccounts-1, false)

	var action FailoverAction
	for i := 0; i < numAccounts; i++ {
		accountID := int64(1000 + i)
		action = fs.HandleFailoverError(context.Background(), mock, accountID, service.PlatformAnthropic, maxSameAccountRetries, transportErrFailoverErr())
	}
	require.Equal(t, FailoverExhausted, action, "every account failing with a transport error must exhaust failover")
	require.Len(t, fs.FailedAccountIDs, numAccounts, "each dead account should be recorded exactly once")
	require.Empty(t, mock.calls, "transport errors are not RetryableOnSameAccount, so HandleFailoverError's own TempUnschedule path is not the mechanism here (the bench happens inside Forward itself)")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &GatewayHandler{}
	h.handleFailoverExhausted(c, fs.LastFailoverErr, service.PlatformAnthropic, false)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Equal(t, 1, strings.Count(rec.Body.String(), `"type":"error"`),
		"exactly one error response must be written, not a concatenation of per-account failures")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "error", payload["type"])
	errObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "upstream_error", errObj["type"])
}
