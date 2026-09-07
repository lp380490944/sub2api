package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 端到端不变量（fork 指纹功能的回归护栏）：
//
//  1. 出站 User-Agent 头里的版本 == body billing 块 cc_version 的 X.Y.Z 前缀；
//  2. cc_version 的 .fp 后缀按该版本重算；
//  3. mimic 路径下账号注册指纹的 OS/Arch 真正到达出站头（而不是被 DefaultHeaders 覆盖回 Linux/arm64）；
//  4. 非 claude-cli 客户端不读不写账号级持久缓存。
//
// 2026-09-07 生产复现过 1/2 的反例：账号 12/14 头 2.1.263、体 2.1.261/2.1.260——
// 移植指纹生成时漏了"指纹压过 DefaultHeaders"这一步。本测试就是为它写的。
func TestMimicPath_FingerprintReachesWireAndBillingIsConsistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetGatewayForwardingSettingsCacheForTest(t)

	origVersion := claude.CLIVersion()
	t.Cleanup(func() { claude.SetCLICurrentVersion(origVersion) })
	require.True(t, claude.SetCLICurrentVersion("2.1.963"))
	SetCachedRecentVersions([]string{"2.1.963", "2.1.961", "2.1.960"})
	t.Cleanup(func() { SetCachedRecentVersions(nil) })

	ccVersionRe := regexp.MustCompile(`cc_version=(\d+\.\d+\.\d+)\.([0-9a-f]{3})`)

	for _, endpoint := range []string{"messages", "count_tokens"} {
		// 生产账号 ID 的分桶：1/6 → recent[0]，12 → recent[1]，14 → recent[2]。
		for _, tc := range []struct {
			accountID int64
			wantVer   string
			regOS     string
			regArch   string
			wantOS    string
			wantArch  string
		}{
			{accountID: 1, wantVer: "2.1.963", wantOS: "Linux", wantArch: "arm64"},
			{accountID: 6, wantVer: "2.1.963", regOS: "MacOS", regArch: "arm64", wantOS: "MacOS", wantArch: "arm64"},
			{accountID: 12, wantVer: "2.1.961", regOS: "MacOS", regArch: "x64", wantOS: "MacOS", wantArch: "x64"},
			{accountID: 14, wantVer: "2.1.960", regOS: "Windows", regArch: "x64", wantOS: "Windows", wantArch: "x64"},
		} {
			t.Run(fmt.Sprintf("%s/account_%d_%s", endpoint, tc.accountID, tc.wantVer), func(t *testing.T) {
				cache := &trackingIdentityCache{}
				svc := &GatewayService{cfg: &config.Config{}, identityService: NewIdentityService(cache)}

				account := &Account{ID: tc.accountID, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
					Credentials: map[string]any{"access_token": "oauth-tok"},
					Status:      StatusActive, Schedulable: true, Extra: map[string]any{},
				}
				if tc.regOS != "" {
					account.Extra[ExtraKeyRegistrationFingerprint] = &RegistrationFingerprint{OS: tc.regOS, Arch: tc.regArch}
				}

				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				// NewAPI 透传过来的非 CC 客户端；顺带塞一份伪造的 stainless 头，验证不会漏到上游。
				c.Request.Header.Set("User-Agent", "Go-http-client/1.1")
				c.Request.Header.Set("X-Stainless-OS", "FreeBSD")
				c.Request.Header.Set("X-Stainless-Arch", "riscv")

				// gateway_forward 在 mimic 路径会先重写 system 注入 billing 块。
				body := []byte(`{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hello world"}]}`)
				body = rewriteSystemForNonClaudeCodeWithPromptBlocks(body, nil, "", "")

				var (
					req      *http.Request
					wireBody []byte
					err      error
				)
				if endpoint == "messages" {
					req, wireBody, err = svc.buildUpstreamRequest(context.Background(), c, account, body,
						"oauth-tok", "oauth", "claude-sonnet-5", true, true)
				} else {
					req, wireBody, err = svc.buildCountTokensRequest(context.Background(), c, account, body,
						"oauth-tok", "oauth", "claude-sonnet-5", true)
				}
				require.NoError(t, err)
				defer func() { require.NoError(t, req.Body.Close()) }()

				ua := getHeaderRaw(req.Header, "User-Agent")
				require.Equal(t, "claude-cli/"+tc.wantVer+" (external, cli)", ua, "wire UA")
				require.Equal(t, tc.wantOS, getHeaderRaw(req.Header, "X-Stainless-OS"), "wire x-stainless-os")
				require.Equal(t, tc.wantArch, getHeaderRaw(req.Header, "X-Stainless-Arch"), "wire x-stainless-arch")
				require.Equal(t, "node", getHeaderRaw(req.Header, "X-Stainless-Runtime"))
				require.Equal(t, "cli", getHeaderRaw(req.Header, "X-App"))
				// 头去重：同一 header 不能以两种大小写各出现一次
				for k := range req.Header {
					for k2 := range req.Header {
						if k != k2 && strings.EqualFold(k, k2) {
							t.Fatalf("duplicate header forms on wire: %q and %q", k, k2)
						}
					}
				}

				var billing string
				for _, item := range gjson.GetBytes(wireBody, "system").Array() {
					if txt := item.Get("text").String(); strings.HasPrefix(txt, "x-anthropic-billing-header") {
						billing = txt
					}
				}
				require.NotEmpty(t, billing, "billing block must be injected on mimic path")
				m := ccVersionRe.FindStringSubmatch(billing)
				require.Len(t, m, 3, "cc_version with fingerprint suffix, got %q", billing)
				require.Equal(t, ExtractCLIVersion(ua), m[1], "body cc_version must equal wire UA version")
				require.Equal(t, computeClaudeCodeFingerprint(wireBody, m[1]), m[2], "fp suffix must be recomputed for the synced version")

				actual, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Equal(t, wireBody, actual)

				require.Equal(t, 0, cache.setCalls, "non-CLI client must not persist an account fingerprint")
			})
		}
	}
}

// 真 CC 客户端路径不受影响：走缓存指纹（含版本下限抬升），UA 与 cc_version 仍一致。
func TestClaudeCodePath_CachedFingerprintFlooredAndConsistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetGatewayForwardingSettingsCacheForTest(t)

	origVersion := claude.CLIVersion()
	t.Cleanup(func() { claude.SetCLICurrentVersion(origVersion) })
	require.True(t, claude.SetCLICurrentVersion("2.1.963"))

	// 生产形态：账号 6 的缓存卡在 2.1.237，客户端更旧（2.1.153）→ 抬到 tracker 版本。
	cache := &trackingIdentityCache{initialFP: &Fingerprint{
		UserAgent: "claude-cli/2.1.237 (external, cli)", ClientID: "cid", StainlessOS: "Darwin", StainlessArch: "arm64",
	}}
	svc := &GatewayService{cfg: &config.Config{}, identityService: NewIdentityService(cache)}
	account := &Account{ID: 6, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-tok"}, Status: StatusActive, Schedulable: true}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.153 (external, cli)")

	body := []byte(`{"model":"claude-fable-5-1","system":[{"type":"text","text":""}],"messages":[{"role":"user","content":"hello"}]}`)
	billing, err := buildBillingAttributionText(body, "2.1.153")
	require.NoError(t, err)
	body = []byte(strings.Replace(string(body), `"text":""`, `"text":"`+billing+`"`, 1))

	req, wireBody, err := svc.buildUpstreamRequest(context.Background(), c, account, body,
		"oauth-tok", "oauth", "claude-fable-5-1", true, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, req.Body.Close()) }()

	ua := getHeaderRaw(req.Header, "User-Agent")
	require.Equal(t, "claude-cli/2.1.963 (external, cli)", ua, "cached UA must be floored to the tracker version")
	require.Equal(t, "Darwin", getHeaderRaw(req.Header, "X-Stainless-OS"), "stainless fields keep cached values")
	require.NotNil(t, cache.stored)
	require.Equal(t, "claude-cli/2.1.963 (external, cli)", cache.stored.UserAgent, "floored UA persisted")

	text := gjson.GetBytes(wireBody, "system.0.text").String()
	require.Contains(t, text, "cc_version=2.1.963."+computeClaudeCodeFingerprint(wireBody, "2.1.963")+";")
}
