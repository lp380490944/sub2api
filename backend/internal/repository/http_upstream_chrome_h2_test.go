package repository

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// 建连阶段失败（代理不可达）必须回退到普通 Do，且请求体在回退后仍可重发。
func TestDoWithTLS_ChromeH2_SetupFailureFallsBackToDo(t *testing.T) {
	svc := NewHTTPUpstream(&config.Config{}).(*httpUpstreamService)

	body := []byte(`{"input":"hi"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://127.0.0.1:1/v1/responses", bytes.NewReader(body))
	require.NoError(t, err)
	// 去掉 GetBody，模拟最坏情况：回退前必须自行缓冲
	req.GetBody = nil

	_, err = svc.DoWithTLS(req, "socks5://127.0.0.1:1", 11, 4, tlsfingerprint.ChromeH2Profile())
	require.Error(t, err)
	require.False(t, tlsfingerprint.IsSetupError(err), "error must come from the fallback Do path, not the chrome-h2 setup: %v", err)

	// 回退后请求体已被重建为可读
	require.NotNil(t, req.GetBody)
	rc, err := req.GetBody()
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

// 非 chrome-h2 的 profile 不受影响（nil → Do）。
func TestDoWithTLS_NilProfileUnchanged(t *testing.T) {
	svc := NewHTTPUpstream(&config.Config{}).(*httpUpstreamService)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1/", nil)
	_, err := svc.DoWithTLS(req, "", 1, 1, nil)
	require.Error(t, err)
	require.False(t, tlsfingerprint.IsSetupError(err))
}
