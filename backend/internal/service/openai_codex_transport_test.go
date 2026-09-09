package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func codexTransportTestAccount(extra map[string]any) *Account {
	return &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4,
		Credentials: map[string]any{"access_token": "tok", "chatgpt_account_id": "acct"}, Extra: extra}
}

func TestAccount_CodexTransport(t *testing.T) {
	require.True(t, codexTransportTestAccount(map[string]any{"codex_transport": "chrome-h2"}).IsCodexChromeH2Transport())
	require.True(t, codexTransportTestAccount(map[string]any{"codex_transport": " Chrome-H2 "}).IsCodexChromeH2Transport())
	require.False(t, codexTransportTestAccount(map[string]any{}).IsCodexChromeH2Transport())
	require.False(t, codexTransportTestAccount(nil).IsCodexChromeH2Transport())
	require.False(t, codexTransportTestAccount(map[string]any{"codex_transport": "off"}).IsCodexChromeH2Transport())
	apiKey := codexTransportTestAccount(map[string]any{"codex_transport": "chrome-h2"})
	apiKey.Type = AccountTypeAPIKey
	require.False(t, apiKey.IsCodexChromeH2Transport(), "API key accounts never use the Codex transport")
	anthropic := codexTransportTestAccount(map[string]any{"codex_transport": "chrome-h2"})
	anthropic.Platform = PlatformAnthropic
	require.False(t, anthropic.IsCodexChromeH2Transport())
}

func TestDoOpenAIUpstream_DispatchesChromeH2OnlyWhenEnabled(t *testing.T) {
	newReq := func() *http.Request {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", strings.NewReader("{}"))
		return req
	}
	okResp := func() *http.Response {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}
	}

	upstream := &httpUpstreamRecorder{resp: okResp()}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	_, err := svc.doOpenAIUpstream(newReq(), "socks5://p:1080", codexTransportTestAccount(map[string]any{"codex_transport": "chrome-h2"}))
	require.NoError(t, err)
	require.NotNil(t, upstream.lastProfile)
	require.Equal(t, tlsfingerprint.ChromeH2ProfileName, upstream.lastProfile.Name)
	require.Equal(t, "socks5://p:1080", upstream.lastProxyURL)

	upstream2 := &httpUpstreamRecorder{resp: okResp()}
	svc2 := &OpenAIGatewayService{httpUpstream: upstream2}
	_, err = svc2.doOpenAIUpstream(newReq(), "", codexTransportTestAccount(map[string]any{}))
	require.NoError(t, err)
	require.Nil(t, upstream2.lastProfile, "default accounts keep the plain transport")
}
