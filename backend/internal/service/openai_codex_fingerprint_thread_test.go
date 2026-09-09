package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	threadTestSeed   = "829be25c-6dea-41fe-acd2-b5a85917591b"
	threadTestSeedB  = "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	threadTestClient = "01a07c73-e312-76e1-9054-e4722b79a205" // 真实 Codex 形态：UUIDv7
	threadTestTurn   = "01a07c73-e3a0-7ae1-8000-000000000001"
)

func threadTestAccount(id int64, seed string) *Account {
	return &Account{
		ID: id, Name: "thread-test", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 5,
		Credentials: map[string]any{"access_token": "tok", "chatgpt_account_id": "acct-uuid", "chatgpt_user_id": "user-uuid"},
		Extra:       map[string]any{"codex_fingerprint_mode": "thread", "codex_fingerprint_seed": seed},
	}
}

// ---------------------------------------------------------------------------
// 派生
// ---------------------------------------------------------------------------

func TestDeriveCodexThreadUUIDv7_ShapeAndDeterminism(t *testing.T) {
	orig, ok := parseUUIDv7(threadTestClient)
	require.True(t, ok)
	ms := uuidV7UnixMs(orig)

	derived := deriveCodexThreadUUIDv7(threadTestSeed, codexThreadKindThread, threadTestClient, ms)
	parsed, err := uuid.Parse(derived)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version(), "version nibble must be 7")
	require.Equal(t, uuid.RFC4122, parsed.Variant())
	require.Equal(t, threadTestClient[:13], derived[:13], "48-bit timestamp prefix (first 12 hex + dash) must be preserved")
	require.Equal(t, ms, uuidV7UnixMs(parsed))
	require.NotEqual(t, threadTestClient, derived, "must differ from the client's own value")

	// 确定性 / 跨 seed / 跨 kind
	require.Equal(t, derived, deriveCodexThreadUUIDv7(threadTestSeed, codexThreadKindThread, threadTestClient, ms))
	require.NotEqual(t, derived, deriveCodexThreadUUIDv7(threadTestSeedB, codexThreadKindThread, threadTestClient, ms))
	require.NotEqual(t, derived, deriveCodexThreadUUIDv7(threadTestSeed, codexThreadKindContextWindow, threadTestClient, ms))

	// 回退时间戳不随时间变化且落在纪元之后一年内
	fb1 := codexThreadFallbackUnixMs(threadTestSeed, codexThreadKindThread, "compat-key")
	fb2 := codexThreadFallbackUnixMs(threadTestSeed, codexThreadKindThread, "compat-key")
	require.Equal(t, fb1, fb2)
	require.GreaterOrEqual(t, fb1, codexThreadEpochMs)
	require.Less(t, fb1, codexThreadEpochMs+codexThreadFallbackSpanMs)
}

func TestParseUUIDv7_RejectsV4(t *testing.T) {
	_, ok := parseUUIDv7(uuid.NewString())
	require.False(t, ok)
	_, ok = parseUUIDv7("not-a-uuid")
	require.False(t, ok)
}

// ---------------------------------------------------------------------------
// 原值捕获
// ---------------------------------------------------------------------------

func desktopClientMetadata() map[string]any {
	turnMeta := `{"session_id":"` + threadTestClient + `","thread_id":"` + threadTestClient + `","turn_id":"` + threadTestTurn + `","root_turn_id":"` + threadTestTurn + `","installation_id":"7f582abd-05d2-4d1b-9c1e-1234567890ab","window_id":"` + threadTestClient + `:1","window_number":1,"context_window_id":"01a07c73-e312-76e1-9054-e4722b79a2ff","sandbox":"seatbelt","thread_source":"user"}`
	return map[string]any{
		"session_id":              threadTestClient,
		"thread_id":               threadTestClient,
		"turn_id":                 threadTestTurn,
		"root_turn_id":            threadTestTurn,
		"x-codex-installation-id": "7f582abd-05d2-4d1b-9c1e-1234567890ab",
		"x-codex-window-id":       threadTestClient + ":1",
		"x-codex-turn-metadata":   turnMeta,
	}
}

func TestCaptureCodexThreadOriginals_Matrix(t *testing.T) {
	t.Run("desktop_full_client_metadata", func(t *testing.T) {
		body := map[string]any{"prompt_cache_key": threadTestClient, "client_metadata": desktopClientMetadata()}
		o := captureCodexThreadOriginals(body, nil)
		require.Equal(t, threadTestClient, o.promptCacheKey)
		require.Equal(t, threadTestClient, o.sessionID)
		require.Equal(t, threadTestClient, o.threadID)
		require.Equal(t, threadTestTurn, o.turnID)
		require.Equal(t, threadTestClient+":1", o.windowID)
		require.Equal(t, "01a07c73-e312-76e1-9054-e4722b79a2ff", o.contextWindowID)
		require.NotEmpty(t, o.turnMetadataRaw)
		require.Empty(t, o.parentThreadID)
	})
	t.Run("cli_prompt_cache_key_only", func(t *testing.T) {
		o := captureCodexThreadOriginals(map[string]any{"prompt_cache_key": threadTestClient}, nil)
		require.Equal(t, threadTestClient, o.threadInput())
		require.Empty(t, o.turnID)
		require.Empty(t, o.turnMetadataRaw)
	})
	t.Run("empty_body", func(t *testing.T) {
		o := captureCodexThreadOriginals(map[string]any{}, nil)
		require.Empty(t, o.threadInput())
	})
	t.Run("direct_client_headers", func(t *testing.T) {
		h := http.Header{}
		h.Set("thread-id", threadTestClient)
		h.Set("session-id", threadTestClient)
		h.Set(codexParentThreadIDHeader, "01a07c60-1111-7aaa-8000-000000000002")
		h.Set("x-codex-window-id", threadTestClient+":3")
		h.Set("x-codex-turn-metadata", `{"turn_id":"`+threadTestTurn+`","parent_thread_id":"01a07c60-1111-7aaa-8000-000000000002"}`)
		o := captureCodexThreadOriginals(map[string]any{}, h)
		require.Equal(t, threadTestClient, o.threadID)
		require.Equal(t, "01a07c60-1111-7aaa-8000-000000000002", o.parentThreadID)
		require.Equal(t, threadTestClient+":3", o.windowID)
		require.Equal(t, threadTestTurn, o.turnID)
	})
	t.Run("parent_in_metadata_beats_header", func(t *testing.T) {
		h := http.Header{}
		h.Set(codexParentThreadIDHeader, "header-parent")
		cm := desktopClientMetadata()
		cm[codexParentThreadIDHeader] = "01a07c60-1111-7aaa-8000-000000000009"
		o := captureCodexThreadOriginals(map[string]any{"client_metadata": cm}, h)
		require.Equal(t, "01a07c60-1111-7aaa-8000-000000000009", o.parentThreadID)
	})
	t.Run("raw_and_map_agree", func(t *testing.T) {
		body := map[string]any{"prompt_cache_key": threadTestClient, "client_metadata": desktopClientMetadata(), "input": []any{}}
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		require.Equal(t, captureCodexThreadOriginals(body, nil), captureCodexThreadOriginalsRaw(raw, nil))
	})
	t.Run("invalid_turn_metadata_json", func(t *testing.T) {
		cm := desktopClientMetadata()
		cm["x-codex-turn-metadata"] = "{not json"
		o := captureCodexThreadOriginals(map[string]any{"client_metadata": cm}, nil)
		require.Empty(t, o.turnMetadataRaw)
		require.Equal(t, threadTestClient, o.threadID)
	})
}

// ---------------------------------------------------------------------------
// 解析器
// ---------------------------------------------------------------------------

func TestResolveCodexThreadFingerprintIDs_DesktopBody(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	o := captureCodexThreadOriginals(map[string]any{"prompt_cache_key": threadTestClient, "client_metadata": desktopClientMetadata()}, nil)

	ids := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, o)
	require.NotNil(t, ids)
	require.Equal(t, codexFingerprintThread, ids.mode)
	require.Equal(t, int64(11), ids.accountID)

	// 宗旨 2：四者相等；宗旨 3：v7 + 原时间戳
	require.Equal(t, ids.threadID, ids.sessionID)
	require.Equal(t, threadTestClient[:13], ids.threadID[:13])
	require.NotEqual(t, threadTestClient, ids.threadID)
	parsed, err := uuid.Parse(ids.threadID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())

	// turn：原值 v7 → 派生保时间戳；root == turn
	require.Equal(t, threadTestTurn[:13], ids.turnID[:13])
	require.NotEqual(t, threadTestTurn, ids.turnID)
	require.Equal(t, ids.turnID, ids.rootTurnID)

	// window：保留原后缀 :1
	require.Equal(t, 1, ids.windowNumber)
	require.Equal(t, ids.threadID+":1", ids.windowID)

	// installation：v4，同 device 派生
	require.Equal(t, resolveConvergedInstallationID(account, threadTestSeed), ids.installationID)
	inst, err := uuid.Parse(ids.installationID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(4), inst.Version())

	// context_window：v7，沿用原 context_window_id 的时间戳
	require.Equal(t, "01a07c73-e312"[:13], ids.contextWindowID[:13])
	cw, err := uuid.Parse(ids.contextWindowID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), cw.Version())

	// turn-metadata：八键齐全，其余键保留，不注入 turn_started_at_unix_ms
	meta := gjson.Parse(ids.turnMetadataJSON)
	require.True(t, meta.IsObject())
	for _, k := range []string{"session_id", "thread_id", "turn_id", "root_turn_id", "installation_id", "window_id", "window_number", "context_window_id"} {
		require.True(t, meta.Get(k).Exists(), k)
	}
	require.Equal(t, ids.threadID, meta.Get("session_id").String())
	require.Equal(t, ids.threadID, meta.Get("thread_id").String())
	require.Equal(t, ids.turnID, meta.Get("root_turn_id").String())
	require.Equal(t, ids.installationID, meta.Get("installation_id").String())
	require.Equal(t, ids.windowID, meta.Get("window_id").String())
	require.Equal(t, int64(1), meta.Get("window_number").Int())
	require.Equal(t, "seatbelt", meta.Get("sandbox").String())
	require.Equal(t, "user", meta.Get("thread_source").String())
	require.False(t, meta.Get("turn_started_at_unix_ms").Exists())
	require.False(t, meta.Get("parent_thread_id").Exists())
	require.Empty(t, ids.parentThreadID)

	// 去混淆映射
	require.Len(t, ids.exposeReplacements, 2)
	require.Equal(t, codexIdentityReplacement{derived: ids.threadID, original: threadTestClient}, ids.exposeReplacements[0])
	require.Equal(t, codexIdentityReplacement{derived: ids.turnID, original: threadTestTurn}, ids.exposeReplacements[1])

	// 同会话第二轮（新 turn）：thread/session/window/context 相同，turn 不同
	o2 := o
	o2.turnID = "01a07c74-0000-7000-8000-000000000002"
	ids2 := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, o2)
	require.Equal(t, ids.threadID, ids2.threadID)
	require.Equal(t, ids.windowID, ids2.windowID)
	require.Equal(t, ids.contextWindowID, ids2.contextWindowID)
	require.NotEqual(t, ids.turnID, ids2.turnID)

	// 不同账号同原值 → 不同派生
	idsB := svc.resolveCodexThreadFingerprintIDs(context.Background(), threadTestAccount(12, threadTestSeedB), o)
	require.NotEqual(t, ids.threadID, idsB.threadID)
	require.Equal(t, threadTestClient[:13], idsB.threadID[:13])
}

func TestResolveCodexThreadFingerprintIDs_ParentThreadAndWindowDefault(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	parent := "01a07c60-1111-7aaa-8000-000000000002"
	cm := desktopClientMetadata()
	cm[codexParentThreadIDHeader] = parent
	delete(cm, "x-codex-window-id")
	cm["x-codex-turn-metadata"] = `{"parent_thread_id":"` + parent + `","sandbox":"seccomp"}`
	o := captureCodexThreadOriginals(map[string]any{"prompt_cache_key": threadTestClient, "client_metadata": cm}, nil)

	ids := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, o)
	require.NotNil(t, ids)
	require.Equal(t, 0, ids.windowNumber, "no window suffix → :0 (CPA default)")
	require.Equal(t, ids.threadID+":0", ids.windowID)
	require.NotEmpty(t, ids.parentThreadID)
	require.Equal(t, parent[:13], ids.parentThreadID[:13])
	// 父会话自己作为主线程派生时得到同一值
	parentAsMain := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, codexThreadOriginals{promptCacheKey: parent})
	require.Equal(t, parentAsMain.threadID, ids.parentThreadID)
	meta := gjson.Parse(ids.turnMetadataJSON)
	require.Equal(t, ids.parentThreadID, meta.Get("parent_thread_id").String())
	require.Equal(t, "seccomp", meta.Get("sandbox").String())
}

func TestResolveCodexThreadFingerprintIDs_NoSeedIsNil(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	delete(account.Extra, "codex_fingerprint_seed")
	require.Nil(t, svc.resolveCodexThreadFingerprintIDs(context.Background(), account, codexThreadOriginals{promptCacheKey: threadTestClient}))
}

func TestCodexFingerprintMode_ThreadGating(t *testing.T) {
	require.Equal(t, codexFingerprintThread, codexFingerprintModeFromExtra(map[string]any{"codex_fingerprint_mode": "thread"}))
	require.True(t, codexFingerprintModeRequiresSeed(codexFingerprintThread))
	require.True(t, ShouldEnsureCodexFingerprintSeedForExtraUpdates(map[string]any{"codex_fingerprint_mode": "thread"}))
	prepared := prepareCodexFingerprintExtraForCreate(PlatformOpenAI, AccountTypeOAuth, map[string]any{"codex_fingerprint_mode": "thread"})
	_, ok := codexFingerprintSeed(prepared)
	require.True(t, ok, "create must mint a seed for thread mode")
}

// ---------------------------------------------------------------------------
// 非 v7 原值：存储与回退
// ---------------------------------------------------------------------------

type threadIDStoreStub struct {
	stubGatewayCacheForThread
	values map[string]string
	getErr error
	setErr error
	sets   int
}

// stubGatewayCacheForThread 满足 GatewayCache 的最小实现（其余方法未使用）。
type stubGatewayCacheForThread struct{ GatewayCache }

func (s *threadIDStoreStub) GetCodexThreadID(_ context.Context, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	return s.values[key], nil
}

func (s *threadIDStoreStub) SetCodexThreadID(_ context.Context, key, value string, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.sets++
	if _, exists := s.values[key]; !exists {
		s.values[key] = value
	}
	return nil
}

func TestLookupOrCreateCodexThreadID_StoreAndFallback(t *testing.T) {
	account := threadTestAccount(11, threadTestSeed)
	compatKey := "compat-chat-session-key" // 非 v7 原值（chat 桥派生的 prompt_cache_key 形态）

	t.Run("nil_cache_deterministic_fallback", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		a := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		b := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		require.Equal(t, a, b)
		parsed, err := uuid.Parse(a)
		require.NoError(t, err)
		require.Equal(t, uuid.Version(7), parsed.Version())
		require.Equal(t, deriveCodexThreadUUIDv7(threadTestSeed, codexThreadKindThread, compatKey, codexThreadFallbackUnixMs(threadTestSeed, codexThreadKindThread, compatKey)), a)
	})
	t.Run("store_miss_then_hit", func(t *testing.T) {
		store := &threadIDStoreStub{values: map[string]string{}}
		svc := &OpenAIGatewayService{cache: store}
		first := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		require.Equal(t, 1, store.sets)
		// 清掉 L1 后仍从 Redis 命中同一值
		svc.openaiCodexThreadIDs.Range(func(k, _ any) bool { svc.openaiCodexThreadIDs.Delete(k); return true })
		second := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		require.Equal(t, first, second)
		require.Equal(t, 1, store.sets, "hit must not rewrite")
		// L1 命中路径
		third := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		require.Equal(t, first, third)
	})
	t.Run("store_setnx_race_reads_back_winner", func(t *testing.T) {
		store := &threadIDStoreStub{values: map[string]string{codexThreadIDStoreKey(11, compatKey): "01a0ffff-0000-7000-8000-00000000abcd"}}
		svc := &OpenAIGatewayService{cache: store}
		require.Equal(t, "01a0ffff-0000-7000-8000-00000000abcd", svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey))
	})
	t.Run("store_error_falls_back", func(t *testing.T) {
		store := &threadIDStoreStub{values: map[string]string{}, getErr: errors.New("redis down")}
		svc := &OpenAIGatewayService{cache: store}
		got := svc.lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey)
		require.Equal(t, (&OpenAIGatewayService{}).lookupOrCreateCodexThreadID(context.Background(), account, threadTestSeed, compatKey), got)
	})
}

// ---------------------------------------------------------------------------
// 头 / 体应用
// ---------------------------------------------------------------------------

func TestApplyCodexFingerprintHeaders_ThreadMode(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	ids := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, codexThreadOriginals{promptCacheKey: threadTestClient})

	h := http.Header{}
	h.Set("session_id", "292762f8c08b5da8")
	h.Set("conversation_id", "292762f8c08b5da8")
	h.Set(codexParentThreadIDHeader, "stale")
	applyCodexFingerprintHeaders(h, ids)

	require.Equal(t, ids.threadID, h.Get("session-id"))
	require.Equal(t, ids.threadID, h.Get("thread-id"))
	require.Equal(t, ids.threadID, h.Get("x-client-request-id"))
	require.Equal(t, ids.installationID, h.Get("x-codex-installation-id"))
	require.Equal(t, ids.threadID+":0", h.Get("x-codex-window-id"))
	require.Equal(t, ids.turnMetadataJSON, h.Get("x-codex-turn-metadata"))
	require.Empty(t, h.Get("session_id"))
	require.Empty(t, h.Get("conversation_id"))
	require.Empty(t, h.Get(codexParentThreadIDHeader))
}

func TestApplyCodexFingerprintBody_ThreadMode_MapAndRawAgree(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	body := map[string]any{"model": "gpt-5.6-sol", "prompt_cache_key": threadTestClient, "client_metadata": desktopClientMetadata(), "input": []any{"hi"}}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	o := captureCodexThreadOriginals(body, nil)
	ids := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, o)

	require.True(t, applyCodexFingerprintClientMetadata(body, ids))
	rawOut, changed, err := applyCodexFingerprintClientMetadataRaw(raw, ids)
	require.NoError(t, err)
	require.True(t, changed)

	var fromRaw map[string]any
	require.NoError(t, json.Unmarshal(rawOut, &fromRaw))
	require.Equal(t, body["client_metadata"], fromRaw["client_metadata"])
	require.Equal(t, ids.threadID, body["prompt_cache_key"])
	require.Equal(t, ids.threadID, fromRaw["prompt_cache_key"])

	cm, ok := body["client_metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, ids.threadID, cm["session_id"])
	require.Equal(t, ids.threadID, cm["thread_id"])
	require.Equal(t, ids.turnID, cm["turn_id"])
	require.Equal(t, ids.turnID, cm["root_turn_id"])
	require.Equal(t, ids.installationID, cm["x-codex-installation-id"])
	require.Equal(t, ids.windowID, cm["x-codex-window-id"])
	require.Equal(t, ids.turnMetadataJSON, cm["x-codex-turn-metadata"])

	// 缺失 prompt_cache_key / client_metadata 时注入
	bare := map[string]any{"model": "gpt-5.6-sol"}
	require.True(t, applyCodexFingerprintClientMetadata(bare, ids))
	require.Equal(t, ids.threadID, bare["prompt_cache_key"])
	require.NotNil(t, bare["client_metadata"])
}

func TestRestoreCodexFingerprintIDsInPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}
	account := threadTestAccount(11, threadTestSeed)
	ids := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, codexThreadOriginals{promptCacheKey: threadTestClient, turnID: threadTestTurn})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	stageCodexFingerprintIDs(c, ids)

	payload := []byte(`{"prompt_cache_key":"` + ids.threadID + `","window":"` + ids.threadID + `:0","turn":"` + ids.turnID + `"}`)
	restored := restoreCodexFingerprintIDsInPayload(c, account, payload)
	require.Equal(t, `{"prompt_cache_key":"`+threadTestClient+`","window":"`+threadTestClient+`:0","turn":"`+threadTestTurn+`"}`, string(restored))

	// 其它账号 / 无 staged → no-op
	other := threadTestAccount(12, threadTestSeedB)
	require.Equal(t, payload, restoreCodexFingerprintIDsInPayload(c, other, payload))
	stageCodexFingerprintIDs(c, nil)
	require.Equal(t, payload, restoreCodexFingerprintIDsInPayload(c, account, payload))
}

// ---------------------------------------------------------------------------
// 端到端：完整 Forward() 管线
// ---------------------------------------------------------------------------

func threadE2EUpstreamJSON(promptCacheKey string) *http.Response {
	body := `{"id":"resp_thread_e2e","object":"response","status":"completed","prompt_cache_key":"` + promptCacheKey + `","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func threadE2EForward(t *testing.T, account *Account, body []byte, upstream *httpUpstreamRecorder, svc *OpenAIGatewayService) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Go-http-client/1.1") // 中继形态：无任何 Codex 身份头
	c.Set("api_key_id", int64(2))
	c.Set("user_id", int64(1))
	_, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	return c, rec
}

func newThreadE2EService(upstream *httpUpstreamRecorder) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false, AllowInsecureHTTP: true}}},
		httpUpstream: upstream,
	}
}

func assertThreadWireInvariants(t *testing.T, req *http.Request, body []byte, account *Account) *codexFingerprintIDs {
	t.Helper()
	h := req.Header
	thread := getHeaderRaw(h, "thread-id")
	require.NotEmpty(t, thread)
	parsed, err := uuid.Parse(thread)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version(), "thread must be v7")

	// 宗旨 2：四头 + 体相等
	require.Equal(t, thread, getHeaderRaw(h, "session-id"))
	require.Equal(t, thread, getHeaderRaw(h, "x-client-request-id"))
	require.Equal(t, thread, gjson.GetBytes(body, "prompt_cache_key").String())
	require.Equal(t, thread, gjson.GetBytes(body, "client_metadata.session_id").String())
	require.Equal(t, thread, gjson.GetBytes(body, "client_metadata.thread_id").String())
	require.True(t, strings.HasPrefix(getHeaderRaw(h, "x-codex-window-id"), thread+":"))
	require.Equal(t, getHeaderRaw(h, "x-codex-window-id"), gjson.GetBytes(body, "client_metadata.x-codex-window-id").String())

	// 宗旨 4：无下划线头
	require.Empty(t, getHeaderRaw(h, "session_id"))
	require.Empty(t, getHeaderRaw(h, "conversation_id"))

	// installation v4 且头体一致；turn-metadata 头体同一份
	inst := getHeaderRaw(h, "x-codex-installation-id")
	instParsed, err := uuid.Parse(inst)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(4), instParsed.Version())
	require.Equal(t, inst, gjson.GetBytes(body, "client_metadata.x-codex-installation-id").String())
	meta := getHeaderRaw(h, "x-codex-turn-metadata")
	require.NotEmpty(t, meta)
	require.Equal(t, meta, gjson.GetBytes(body, "client_metadata.x-codex-turn-metadata").String())
	require.Equal(t, thread, gjson.Get(meta, "thread_id").String())
	require.Equal(t, gjson.GetBytes(body, "client_metadata.turn_id").String(), gjson.Get(meta, "root_turn_id").String())
	require.Equal(t, gjson.GetBytes(body, "client_metadata.turn_id").String(), gjson.GetBytes(body, "client_metadata.root_turn_id").String())

	// 宗旨 5：身份三元组仍是规范 TUI
	require.Equal(t, "codex-tui", getHeaderRaw(h, "originator"))
	require.True(t, strings.HasPrefix(getHeaderRaw(h, "user-agent"), "codex-tui/"))
	require.NotEmpty(t, getHeaderRaw(h, "version"))

	// 命名空间 v4 值不得残留在体内
	for _, k := range []string{"session_id", "thread_id", "turn_id"} {
		v := gjson.GetBytes(body, "client_metadata."+k).String()
		p, err := uuid.Parse(v)
		require.NoError(t, err, k)
		require.Equal(t, uuid.Version(7), p.Version(), k)
	}
	return &codexFingerprintIDs{threadID: thread, installationID: inst}
}

func TestThreadModeE2E_DesktopBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cm, _ := json.Marshal(desktopClientMetadata())
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"input":[{"role":"user","content":"hello world"}],"prompt_cache_key":"` + threadTestClient + `","client_metadata":` + string(cm) + `}`)
	account := threadTestAccount(11, threadTestSeed)

	upstream := &httpUpstreamRecorder{}
	svc := newThreadE2EService(upstream)
	expected := svc.resolveCodexThreadFingerprintIDs(context.Background(), account, captureCodexThreadOriginalsRaw(body, nil))
	upstream.responses = []*http.Response{threadE2EUpstreamJSON(expected.threadID)}

	_, rec := threadE2EForward(t, account, body, upstream, svc)
	got := assertThreadWireInvariants(t, upstream.lastReq, upstream.lastBody, account)

	// 宗旨 3：时间戳前缀沿用客户端原值；派生值确定
	require.Equal(t, expected.threadID, got.threadID)
	require.Equal(t, threadTestClient[:13], got.threadID[:13])
	require.Equal(t, expected.threadID+":1", getHeaderRaw(upstream.lastReq.Header, "x-codex-window-id"), "client window suffix :1 preserved")
	require.Equal(t, threadTestTurn[:13], gjson.GetBytes(upstream.lastBody, "client_metadata.turn_id").String()[:13])

	// 去混淆：上游回显派生 id，客户端收到原值
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, threadTestClient, gjson.GetBytes(rec.Body.Bytes(), "prompt_cache_key").String())
	require.NotContains(t, rec.Body.String(), expected.threadID)
}

func TestThreadModeE2E_CLIBody_TwoTurns_TwoAccounts_Failover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"input":[{"role":"user","content":"hello world"}],"prompt_cache_key":"` + threadTestClient + `"}`)
	account := threadTestAccount(11, threadTestSeed)

	upstream := &httpUpstreamRecorder{responses: []*http.Response{threadE2EUpstreamJSON("x"), threadE2EUpstreamJSON("x")}}
	svc := newThreadE2EService(upstream)

	// 第一轮
	threadE2EForward(t, account, body, upstream, svc)
	first := assertThreadWireInvariants(t, upstream.requests[0], upstream.bodies[0], account)
	firstTurn := gjson.GetBytes(upstream.bodies[0], "client_metadata.turn_id").String()
	firstCtx := gjson.Get(getHeaderRaw(upstream.requests[0].Header, "x-codex-turn-metadata"), "context_window_id").String()
	require.Equal(t, first.threadID+":0", getHeaderRaw(upstream.requests[0].Header, "x-codex-window-id"), "CLI body has no window → :0")

	// 第二轮（同会话）：thread/window/context 相同，turn 不同
	threadE2EForward(t, account, body, upstream, svc)
	second := assertThreadWireInvariants(t, upstream.requests[1], upstream.bodies[1], account)
	require.Equal(t, first.threadID, second.threadID)
	require.Equal(t, getHeaderRaw(upstream.requests[0].Header, "x-codex-window-id"), getHeaderRaw(upstream.requests[1].Header, "x-codex-window-id"))
	require.Equal(t, firstCtx, gjson.Get(getHeaderRaw(upstream.requests[1].Header, "x-codex-turn-metadata"), "context_window_id").String())
	require.NotEqual(t, firstTurn, gjson.GetBytes(upstream.bodies[1], "client_metadata.turn_id").String())

	// failover 到另一账号：同一 gin context 重新 Forward，派生必须不同且不残留
	other := threadTestAccount(12, threadTestSeedB)
	upstream2 := &httpUpstreamRecorder{responses: []*http.Response{threadE2EUpstreamJSON("x")}}
	svc2 := newThreadE2EService(upstream2)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key_id", int64(2))
	stageCodexFingerprintIDs(c, &codexFingerprintIDs{accountID: 11, mode: codexFingerprintThread, threadID: first.threadID})
	_, err := svc2.Forward(context.Background(), c, other, body)
	require.NoError(t, err)
	third := assertThreadWireInvariants(t, upstream2.lastReq, upstream2.lastBody, other)
	require.NotEqual(t, first.threadID, third.threadID)
	require.Equal(t, threadTestClient[:13], third.threadID[:13])
	require.NotEqual(t, first.installationID, third.installationID)
}

func TestThreadModeE2E_EmptyBodyStillConsistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"input":[{"role":"user","content":"hello world"}]}`)
	account := threadTestAccount(11, threadTestSeed)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{threadE2EUpstreamJSON("x"), threadE2EUpstreamJSON("x")}}
	svc := newThreadE2EService(upstream)

	threadE2EForward(t, account, body, upstream, svc)
	first := assertThreadWireInvariants(t, upstream.requests[0], upstream.bodies[0], account)
	// 无任何 id 时按内容种子稳定：同一内容第二次得到同一 thread
	threadE2EForward(t, account, body, upstream, svc)
	second := assertThreadWireInvariants(t, upstream.requests[1], upstream.bodies[1], account)
	require.Equal(t, first.threadID, second.threadID)
}

func TestThreadModeE2E_PassthroughAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cm, _ := json.Marshal(desktopClientMetadata())
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"instructions":"You are Codex.","input":[{"role":"user","content":"hello world"}],"prompt_cache_key":"` + threadTestClient + `","client_metadata":` + string(cm) + `}`)
	account := threadTestAccount(11, threadTestSeed)
	account.Extra["openai_passthrough"] = true

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_pt","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}}
	svc := newThreadE2EService(upstream)
	threadE2EForward(t, account, body, upstream, svc)
	got := assertThreadWireInvariants(t, upstream.lastReq, upstream.lastBody, account)
	require.Equal(t, threadTestClient[:13], got.threadID[:13])
}

// 其它模式不受影响：off 模式出站仍无会话头、仍有下划线头（既有行为）。
func TestThreadModeE2E_OffModeUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"input":[{"role":"user","content":"hello world"}],"prompt_cache_key":"` + threadTestClient + `"}`)
	account := threadTestAccount(11, threadTestSeed)
	account.Extra["codex_fingerprint_mode"] = "off"
	upstream := &httpUpstreamRecorder{responses: []*http.Response{threadE2EUpstreamJSON("x")}}
	svc := newThreadE2EService(upstream)
	threadE2EForward(t, account, body, upstream, svc)
	h := upstream.lastReq.Header
	require.Empty(t, getHeaderRaw(h, "thread-id"))
	require.Empty(t, getHeaderRaw(h, "session-id"))
	require.NotEmpty(t, getHeaderRaw(h, "session_id"))
	require.Equal(t, "codex-tui", getHeaderRaw(h, "originator"))
}
