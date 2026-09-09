package tlsfingerprint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// fork：Codex（chatgpt.com）传输层指纹，仿照 CLIProxyAPI helps/utls_client.go 的
// utlsRoundTripper：Chrome ClientHello（utls.HelloChrome_Auto）→ 在 uTLS 连接上跑
// HTTP/2（x/net/http2，默认 SETTINGS）→ 每请求一条新连接，响应体 Close 时关连接。
// 不定制 SETTINGS、不改头顺序、不设 Accept-Encoding——CPA 也没做，本实现不自行发挥。
// 详见 docs/fork/CODEX_IDENTITY_EMULATION.md §3。

// ChromeH2ProfileName 是 DoWithTLS 识别本传输的哨兵 Profile 名。
const ChromeH2ProfileName = "codex-chrome-h2"

// ChromeH2Profile 返回哨兵 Profile：repository 层据 Name 分流到 ChromeH2RoundTripper，
// 不会用它去构造自定义 ClientHello。
func ChromeH2Profile() *Profile {
	return &Profile{Name: ChromeH2ProfileName}
}

// IsChromeH2Profile 判断 profile 是否为本传输的哨兵。
func IsChromeH2Profile(profile *Profile) bool {
	return profile != nil && profile.Name == ChromeH2ProfileName
}

// SetupError 表示请求发出之前（拨号 / 握手 / ALPN / h2 初始化）的失败。
// 调用方只对这类错误回退到普通传输：请求一旦发出就不再重放，避免非幂等请求重复。
type SetupError struct {
	Stage string
	Err   error
}

func (e *SetupError) Error() string { return "chrome-h2 " + e.Stage + ": " + e.Err.Error() }
func (e *SetupError) Unwrap() error { return e.Err }

// IsSetupError 报告 err 是否为建连阶段错误。
func IsSetupError(err error) bool {
	var setupErr *SetupError
	return errors.As(err, &setupErr)
}

var errChromeH2NotNegotiated = errors.New("upstream did not negotiate h2")

// ChromeH2RoundTripper 每次 RoundTrip 建一条新连接（CPA 语义）。
// 无状态，可按代理复用一个实例。
type ChromeH2RoundTripper struct {
	proxyURL *url.URL
	// insecureSkipVerify 仅供本包测试对自签名服务端使用，生产永远为 false。
	insecureSkipVerify bool

	h2Once sync.Once
	h2     *http2.Transport
	h2Err  error
}

// h2Transport 惰性构造一个绑定了底层 *http.Transport 的 http2.Transport：
// Go 1.26+ 的 x/net/http2 通过 net/http 的 ClientConn 实现，NewClientConn 要求先 ConfigureTransports。
func (t *ChromeH2RoundTripper) h2Transport() (*http2.Transport, error) {
	t.h2Once.Do(func() {
		t.h2, t.h2Err = http2.ConfigureTransports(&http.Transport{})
	})
	return t.h2, t.h2Err
}

// NewChromeH2RoundTripper 构造传输；proxyURL 为 nil 表示直连，支持 socks5/socks5h 与 http。
// https 代理由调用方在分流前排除（明文 CONNECT 无法对 https 代理建立隧道）。
func NewChromeH2RoundTripper(proxyURL *url.URL) *ChromeH2RoundTripper {
	return &ChromeH2RoundTripper{proxyURL: proxyURL}
}

func (t *ChromeH2RoundTripper) dialTunnel(ctx context.Context, addr string) (net.Conn, error) {
	if t.proxyURL == nil {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "tcp", addr)
	}
	switch strings.ToLower(t.proxyURL.Scheme) {
	case "socks5", "socks5h":
		return DialSOCKS5Tunnel(ctx, t.proxyURL, addr)
	case "http":
		return DialHTTPConnectTunnel(ctx, t.proxyURL, addr)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", t.proxyURL.Scheme)
	}
}

// createConnection 拨号 → Chrome 握手 → 校验 h2 → 建 http2 ClientConn（对应 CPA createConnection）。
func (t *ChromeH2RoundTripper) createConnection(ctx context.Context, host, addr string) (*http2.ClientConn, error) {
	conn, err := t.dialTunnel(ctx, addr)
	if err != nil {
		return nil, &SetupError{Stage: "dial", Err: err}
	}
	tlsConn := utls.UClient(conn, &utls.Config{ServerName: host, InsecureSkipVerify: t.insecureSkipVerify}, utls.HelloChrome_Auto) //nolint:gosec // 仅测试置 true
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, &SetupError{Stage: "handshake", Err: err}
	}
	state := tlsConn.ConnectionState()
	if state.NegotiatedProtocol != "h2" {
		_ = tlsConn.Close()
		return nil, &SetupError{Stage: "alpn", Err: fmt.Errorf("%w (got %q)", errChromeH2NotNegotiated, state.NegotiatedProtocol)}
	}
	slog.Debug("codex_chrome_h2_handshake",
		"host", host,
		"tls_version", state.Version,
		"cipher_suite", fmt.Sprintf("0x%04x", state.CipherSuite),
		"alpn", state.NegotiatedProtocol)

	h2t, err := t.h2Transport()
	if err != nil {
		_ = tlsConn.Close()
		return nil, &SetupError{Stage: "h2", Err: err}
	}
	h2Conn, err := h2t.NewClientConn(tlsConn)
	if err != nil {
		_ = tlsConn.Close()
		return nil, &SetupError{Stage: "h2", Err: err}
	}
	return h2Conn, nil
}

// RoundTrip 实现 http.RoundTripper。
func (t *ChromeH2RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, &SetupError{Stage: "request", Err: errors.New("nil request")}
	}
	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(host, port)

	h2Conn, err := t.createConnection(req.Context(), host, addr)
	if err != nil {
		return nil, err
	}
	resp, err := h2Conn.RoundTrip(req)
	if err != nil {
		_ = h2Conn.Close()
		return nil, err
	}
	if resp == nil {
		_ = h2Conn.Close()
		return nil, errors.New("chrome-h2: upstream returned an empty response")
	}
	if resp.Body == nil {
		resp.Body = http.NoBody
	}
	resp.Body = &closeConnectionBody{ReadCloser: resp.Body, closeConnection: h2Conn.Close}
	return resp, nil
}

// closeConnectionBody 在响应体关闭时一并关闭底层连接（每请求一连接）。
type closeConnectionBody struct {
	io.ReadCloser
	closeConnection func() error
	once            sync.Once
	err             error
}

func (b *closeConnectionBody) Close() error {
	if b == nil {
		return nil
	}
	b.once.Do(func() {
		// 顺序与 CPA 一致：先关连接再关 body。
		var errConn, errBody error
		if b.closeConnection != nil {
			errConn = b.closeConnection()
		}
		if b.ReadCloser != nil {
			errBody = b.ReadCloser.Close()
		}
		b.err = errors.Join(errBody, errConn)
	})
	return b.err
}
