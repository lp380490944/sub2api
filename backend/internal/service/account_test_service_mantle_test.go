package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// mantleCapturingUpstream records outgoing requests so the test can assert the
// account-test path targets the native Mantle endpoint.
type mantleCapturingUpstream struct {
	requests []*http.Request
	resp     *http.Response
}

func (u *mantleCapturingUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return u.resp, nil
}

func (u *mantleCapturingUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return u.resp, nil
}

func (u *mantleCapturingUpstream) Prewarm(_ context.Context, _ string, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) error {
	return nil
}

// A Bedrock Mantle account must be tested against the native Anthropic
// /v1/messages endpoint with x-api-key auth — NOT the SigV4/InvokeModel path
// (which fails with "aws_access_key_id not found"). Regression test for the
// account-test dispatch that previously routed every IsBedrock() account,
// including Mantle, to testBedrockAccountConnection.
func TestAccountTestService_BedrockMantleUsesNativeMessagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mantle := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
		Credentials: map[string]any{
			"auth_mode":  "bedrock_mantle",
			"aws_region": "eu-north-1",
			"api_key":    "sk-mantle",
		},
	}

	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"content\":[]}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	upstream := &mantleCapturingUpstream{resp: resp}
	svc := &AccountTestService{httpUpstream: upstream}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	_ = svc.testClaudeAccountConnection(c, mantle, "claude-opus-4-8")

	// Before the fix, Mantle hit testBedrockAccountConnection and failed at
	// signer creation, so NO upstream request was made (len == 0).
	require.Len(t, upstream.requests, 1, "Mantle test must make exactly one native /v1/messages request")
	req := upstream.requests[0]
	require.Equal(t, "https://bedrock-mantle.eu-north-1.api.aws/v1/messages", req.URL.String())
	require.Equal(t, "sk-mantle", req.Header.Get("x-api-key"), "Mantle authenticates with x-api-key")
	require.Empty(t, req.Header.Get("Authorization"), "Mantle must not use Bearer/SigV4 auth")
}

// A Claude Platform on AWS account (auth_mode=claude_platform_aws) is also a
// native Anthropic-protocol endpoint (aws-external-anthropic, plain API key +
// workspace id), not SigV4 — the same test-dispatch gap as Bedrock Mantle.
func TestAccountTestService_ClaudePlatformAWSUsesNativeMessagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cpa := &Account{
		ID:       2,
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
		Credentials: map[string]any{
			"auth_mode":    "claude_platform_aws",
			"aws_region":   "us-east-1",
			"api_key":      "sk-cpa",
			"workspace_id": "wrkspc_abc",
		},
	}

	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"content\":[]}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	upstream := &mantleCapturingUpstream{resp: resp}
	svc := &AccountTestService{httpUpstream: upstream}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/2/test", nil)

	_ = svc.testClaudeAccountConnection(c, cpa, "claude-opus-4-8")

	require.Len(t, upstream.requests, 1, "Claude Platform on AWS must make one native /v1/messages request")
	req := upstream.requests[0]
	require.Equal(t, "https://aws-external-anthropic.us-east-1.api.aws/v1/messages?beta=true", req.URL.String())
	require.Equal(t, "sk-cpa", req.Header.Get("x-api-key"))
	require.Equal(t, "wrkspc_abc", req.Header.Get("anthropic-workspace-id"))
	require.Empty(t, req.Header.Get("Authorization"))
}
