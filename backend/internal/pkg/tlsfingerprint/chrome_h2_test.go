//go:build unit

package tlsfingerprint

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

// 本地 h2 TLS 服务端，抓 ClientHello 供断言。
func newH2CaptureServer(t *testing.T) (*httptest.Server, *tls.ClientHelloInfo, *sync.Mutex, *atomic.Int64) {
	t.Helper()
	var (
		mu       sync.Mutex
		captured tls.ClientHelloInfo
		open     atomic.Int64
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proto":"` + r.Proto + `"}`))
	}))
	srv.EnableHTTP2 = true
	srv.TLS = &tls.Config{
		NextProtos: []string{"h2", "http/1.1"},
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			mu.Lock()
			captured = *chi
			mu.Unlock()
			return nil, nil
		},
	}
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			open.Add(1)
		case http.StateClosed:
			open.Add(-1)
		}
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, &captured, &mu, &open
}

func TestChromeH2RoundTripper_ChromeHelloAndH2(t *testing.T) {
	srv, captured, mu, open := newH2CaptureServer(t)
	rt := NewChromeH2RoundTripper(nil)
	rt.insecureSkipVerify = true

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/responses", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "HTTP/2.0", resp.Proto, "must negotiate h2 on the uTLS connection")
	require.Contains(t, string(body), `"proto":"HTTP/2.0"`)

	// ClientHello 必须是 Chrome 预设：ALPN 含 h2；密码套件与 utls Chrome 预设逐项一致（GREASE 值被 Go 服务端剔除后比较）
	mu.Lock()
	chi := *captured
	mu.Unlock()
	require.Contains(t, chi.SupportedProtos, "h2")
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	require.NoError(t, err)
	var want []uint16
	for _, cs := range spec.CipherSuites {
		if cs&0x0f0f == 0x0a0a { // GREASE
			continue
		}
		want = append(want, cs)
	}
	var got []uint16
	for _, cs := range chi.CipherSuites {
		if cs&0x0f0f == 0x0a0a {
			continue
		}
		got = append(got, cs)
	}
	require.Equal(t, want, got, "cipher suites must match utls HelloChrome_Auto preset")
	require.NotEqual(t, tls.CipherSuites()[0].ID, 0, "sanity")

	// 每请求一连接：Body.Close 后服务端连接归零
	require.NoError(t, resp.Body.Close())
	require.Eventually(t, func() bool { return open.Load() == 0 }, 3*time.Second, 20*time.Millisecond, "connection must close with the body")
}

func TestChromeH2RoundTripper_SetupErrors(t *testing.T) {
	// 拨号失败 → SetupError(dial)
	rt := NewChromeH2RoundTripper(nil)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1:1/x", nil)
	_, err := rt.RoundTrip(req)
	require.Error(t, err)
	require.True(t, IsSetupError(err), "dial failure must be a setup error: %v", err)
	require.Contains(t, err.Error(), "dial")

	// 非 h2 服务端 → SetupError(alpn)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	srv.TLS = &tls.Config{NextProtos: []string{"http/1.1"}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	rt2 := NewChromeH2RoundTripper(nil)
	rt2.insecureSkipVerify = true
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	_, err = rt2.RoundTrip(req2)
	require.True(t, IsSetupError(err), "non-h2 upstream must be a setup error: %v", err)
	require.Contains(t, err.Error(), "alpn")

	// 不支持的代理协议 → SetupError(dial)
	bad, _ := url.Parse("ftp://127.0.0.1:1")
	rt3 := NewChromeH2RoundTripper(bad)
	_, err = rt3.RoundTrip(req)
	require.True(t, IsSetupError(err))
	require.True(t, strings.Contains(err.Error(), "unsupported proxy scheme"))
}

func TestChromeH2Profile_Sentinel(t *testing.T) {
	require.True(t, IsChromeH2Profile(ChromeH2Profile()))
	require.False(t, IsChromeH2Profile(nil))
	require.False(t, IsChromeH2Profile(&Profile{Name: "Built-in Default (Node.js 24.x)"}))
}
