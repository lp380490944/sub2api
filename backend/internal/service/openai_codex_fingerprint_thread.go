package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// fork：codex_fingerprint_mode = "thread" —— 按客户端会话仿真真实 Codex 客户端身份。
//
// 生产形态是“Codex 客户端 → 中继（NewAPI）→ sub2api”：中继剥掉全部身份头，只剩请求体
// （prompt_cache_key，Desktop 还带 client_metadata）。off 模式下出站自称 codex-tui 却一个
// 会话头都没有，反而发真实客户端从不发的下划线头，体内 session≠thread、root_turn_id /
// context_window_id 原样透传别人的值。本模式把这些全部按真实客户端形态重建：
//
//   - session-id == thread-id == x-client-request-id == prompt_cache_key
//     == client_metadata.session_id == client_metadata.thread_id（根会话）；
//   - root_turn_id == turn_id；window_id = <thread>:<n>；context_window_id 派生；
//   - 会话类标识全部 UUIDv7，且原值是 v7 时沿用其 48 bit 时间戳；installation 保持 v4（同 device）；
//   - 不发 session_id / conversation_id 下划线头；
//   - originator / user-agent / version 不动，仍由 enforceCodexIdentityHeadersWithUA 收口。
//
// 派生逻辑对应 CLIProxyAPI 的 identity-confuse（按账号确定性派生、头体同源、响应体去混淆），
// 输出形状按真实抓包表。

const (
	codexThreadDeriveVersion = "sub2api:codex-thread-id:v1"
	// codexThreadEpochMs 非 v7 原值且无缓存时的确定性时间戳纪元（2026-01-01T00:00:00Z）。
	codexThreadEpochMs        int64 = 1767225600000
	codexThreadFallbackSpanMs       = int64(365 * 24 * time.Hour / time.Millisecond)
	codexThreadIDStoreTTL           = 7 * 24 * time.Hour
	codexThreadIDL1SweepEvery       = 256

	codexThreadKindThread        = "thread"
	codexThreadKindTurn          = "turn"
	codexThreadKindContextWindow = "context-window"
)

// debugCodexIdentity 由 SUB2API_DEBUG_CODEX_IDENTITY 控制，开启后在出站请求构造末尾
// 打印一行身份头/体摘要（staging 验收用）。取头必须走 getHeaderRaw：出站头以 wire
// 大小写写入，Header.Get 的规范化查找会漏印。
var debugCodexIdentity = parseDebugEnvBool(os.Getenv("SUB2API_DEBUG_CODEX_IDENTITY"))

// ---------------------------------------------------------------------------
// UUIDv7 派生
// ---------------------------------------------------------------------------

// deriveCodexThreadUUIDv7 从账号种子 + 种类 + 客户端原值确定性派生 UUIDv7。
// 前 48 bit = unixMs（原值是 v7 时沿用其时间戳，上游看到的“线程创建时间”与真实客户端一致）；
// 余下随机段由 HMAC-SHA256(seed, version:kind:original) 填充：同账号同原值恒等，
// 不同账号（种子不同）必然不同。对应 CPA 的 codexIdentityConfuseUUID，区别只在输出形状：
// CPA 是 v5，这里按抓包表要求保 v7 + 原时间戳（真实 Codex 全部 Uuid::now_v7，上游派生一律 v4，
// 光看版本位就能认出来）。
func deriveCodexThreadUUIDv7(seed, kind, original string, unixMs int64) string {
	mac := hmac.New(sha256.New, []byte(seed))
	_, _ = mac.Write([]byte(codexThreadDeriveVersion + ":" + kind + ":" + strings.TrimSpace(original)))
	sum := mac.Sum(nil)

	var b [16]byte
	ms := uint64(unixMs) & 0xFFFFFFFFFFFF
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	copy(b[6:], sum[:10])
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return uuid.UUID(b).String()
}

// parseUUIDv7 解析并确认是 UUIDv7（真实 Codex 的 session/thread/turn 都是）。
func parseUUIDv7(value string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Version() != 7 {
		return uuid.Nil, false
	}
	return parsed, true
}

// uuidV7UnixMs 取 UUIDv7 前 48 bit 的毫秒时间戳。
func uuidV7UnixMs(u uuid.UUID) int64 {
	var buf [8]byte
	copy(buf[2:], u[:6])
	return int64(binary.BigEndian.Uint64(buf[:]))
}

// codexThreadFallbackUnixMs 非 v7 原值且缓存不可用时的确定性时间戳：纪元 + HMAC 偏移。
// 禁止 now()——同一原值在多副本/重启后必须派生出同一值。
func codexThreadFallbackUnixMs(seed, kind, original string) int64 {
	mac := hmac.New(sha256.New, []byte(seed))
	_, _ = mac.Write([]byte(codexThreadDeriveVersion + ":fallback-ts:" + kind + ":" + strings.TrimSpace(original)))
	sum := mac.Sum(nil)
	offset := int64(binary.BigEndian.Uint64(sum[:8]) % uint64(codexThreadFallbackSpanMs))
	return codexThreadEpochMs + offset
}

// ---------------------------------------------------------------------------
// 原值捕获
// ---------------------------------------------------------------------------

// codexThreadOriginals 客户端请求里的原始会话标识。必须在账号命名空间改写
// （applyCodexAccountIdentityClientMetadata*）之前捕获，否则拿到的是 v4 形状的派生值。
type codexThreadOriginals struct {
	promptCacheKey   string // body prompt_cache_key
	sessionID        string // client_metadata.session_id
	threadID         string // client_metadata.thread_id → turn-metadata.thread_id → 入站 thread-id / session-id 头
	turnID           string // client_metadata.turn_id → turn-metadata.turn_id
	parentThreadID   string // client_metadata["x-codex-parent-thread-id"] → turn-metadata.parent_thread_id → 入站头
	windowID         string // client_metadata["x-codex-window-id"] → turn-metadata.window_id → 入站头
	contextWindowID  string // turn-metadata.context_window_id
	turnMetadataRaw  string // client_metadata["x-codex-turn-metadata"]（字符串）→ 入站 x-codex-turn-metadata 头
	conversationSeed string // 上述全无时的回退键（explicitOpenAIRequestSessionID → deriveOpenAIContentSessionSeed）
}

// threadInput 返回派生 thread 的会话输入：prompt_cache_key → thread_id → 内容种子。
func (o codexThreadOriginals) threadInput() string {
	for _, candidate := range []string{o.promptCacheKey, o.threadID, o.conversationSeed} {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	return ""
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func trimmedStringFromAny(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// fillCodexThreadOriginalsFromTurnMetadata 用 turn-metadata JSON 补齐尚未捕获的字段。
func (o *codexThreadOriginals) fillFromTurnMetadata(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !gjson.Valid(raw) {
		return
	}
	if o.turnMetadataRaw == "" {
		o.turnMetadataRaw = raw
	}
	meta := gjson.Parse(raw)
	if !meta.IsObject() {
		return
	}
	if o.threadID == "" {
		o.threadID = strings.TrimSpace(meta.Get("thread_id").String())
	}
	if o.sessionID == "" {
		o.sessionID = strings.TrimSpace(meta.Get("session_id").String())
	}
	if o.turnID == "" {
		o.turnID = strings.TrimSpace(meta.Get("turn_id").String())
	}
	if o.parentThreadID == "" {
		o.parentThreadID = strings.TrimSpace(meta.Get("parent_thread_id").String())
	}
	if o.windowID == "" {
		o.windowID = strings.TrimSpace(meta.Get("window_id").String())
	}
	if o.contextWindowID == "" {
		o.contextWindowID = strings.TrimSpace(meta.Get("context_window_id").String())
	}
}

// fillFromHeaders 用入站头补齐（直连 Codex 客户端时才有；经中继一律为空）。
func (o *codexThreadOriginals) fillFromHeaders(h http.Header) {
	if h == nil {
		return
	}
	if o.turnMetadataRaw == "" {
		o.fillFromTurnMetadata(h.Get("x-codex-turn-metadata"))
	}
	if o.threadID == "" {
		o.threadID = firstNonEmptyTrimmed(h.Get("thread-id"), h.Get("session-id"))
	}
	if o.sessionID == "" {
		o.sessionID = strings.TrimSpace(h.Get("session-id"))
	}
	if o.parentThreadID == "" {
		o.parentThreadID = strings.TrimSpace(h.Get(codexParentThreadIDHeader))
	}
	if o.windowID == "" {
		o.windowID = strings.TrimSpace(h.Get("x-codex-window-id"))
	}
}

// captureCodexThreadOriginals 非透传路径（body 已解码为 map）的原值捕获。
func captureCodexThreadOriginals(reqBody map[string]any, clientHeaders http.Header) codexThreadOriginals {
	o := codexThreadOriginals{}
	if reqBody != nil {
		o.promptCacheKey = trimmedStringFromAny(reqBody["prompt_cache_key"])
		if cm, ok := reqBody["client_metadata"].(map[string]any); ok && cm != nil {
			o.sessionID = trimmedStringFromAny(cm["session_id"])
			o.threadID = trimmedStringFromAny(cm["thread_id"])
			o.turnID = trimmedStringFromAny(cm["turn_id"])
			o.parentThreadID = trimmedStringFromAny(cm[codexParentThreadIDHeader])
			o.windowID = trimmedStringFromAny(cm["x-codex-window-id"])
			o.fillFromTurnMetadata(trimmedStringFromAny(cm["x-codex-turn-metadata"]))
		}
	}
	o.fillFromHeaders(clientHeaders)
	return o
}

// captureCodexThreadOriginalsRaw 透传热路径的原值捕获：gjson 只取 client_metadata /
// prompt_cache_key 两个小片段，禁止对可能高达数十 MB 的 body 做全量 Unmarshal。
func captureCodexThreadOriginalsRaw(body []byte, clientHeaders http.Header) codexThreadOriginals {
	o := codexThreadOriginals{}
	if len(body) > 0 {
		root := gjson.ParseBytes(body)
		if root.IsObject() {
			o.promptCacheKey = strings.TrimSpace(root.Get("prompt_cache_key").String())
			if cm := root.Get("client_metadata"); cm.IsObject() {
				o.sessionID = strings.TrimSpace(cm.Get("session_id").String())
				o.threadID = strings.TrimSpace(cm.Get("thread_id").String())
				o.turnID = strings.TrimSpace(cm.Get("turn_id").String())
				o.parentThreadID = strings.TrimSpace(cm.Get(codexParentThreadIDHeader).String())
				o.windowID = strings.TrimSpace(cm.Get("x-codex-window-id").String())
				o.fillFromTurnMetadata(cm.Get("x-codex-turn-metadata").String())
			}
		}
	}
	o.fillFromHeaders(clientHeaders)
	return o
}

// codexThreadConversationSeed 请求体里没有任何会话标识时的回退键，与粘性路由同源
// （显式头/prompt_cache_key → 内容种子），保证同一会话的后续轮次仍落到同一派生 thread。
func codexThreadConversationSeed(c *gin.Context, body []byte) string {
	if seed := explicitOpenAIRequestSessionID(c, body); seed != "" {
		return seed
	}
	return deriveOpenAIContentSessionSeed(body)
}

// ---------------------------------------------------------------------------
// thread id 存储（非 v7 原值时保证跨请求稳定）
// ---------------------------------------------------------------------------

// codexThreadIDStore 是 GatewayCache 实现的可选能力（repository.gatewayCache 已实现），
// 用类型断言探测：GatewayCache 接口本身不加方法，避免改动全部测试 mock；
// 断言失败 / Redis 出错时走确定性回退，永不阻断请求。
type codexThreadIDStore interface {
	GetCodexThreadID(ctx context.Context, key string) (string, error)
	SetCodexThreadID(ctx context.Context, key string, value string, ttl time.Duration) error
}

type codexThreadIDL1Entry struct {
	value     string
	expiresAt time.Time
}

// codexThreadIDStoreKey 按账号 + 原值哈希构造存储键，不含明文会话信息。
func codexThreadIDStoreKey(accountID int64, threadInput string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(threadInput)))
	return fmt.Sprintf("%d:%s", accountID, hex.EncodeToString(sum[:8]))
}

// lookupOrCreateCodexThreadID 为非 v7 原值（或仅有内容种子）解析稳定的派生 thread：
// 进程内 L1 → Redis → 首见时用 now 派生并 SetNX；存储不可用则用确定性回退时间戳派生。
func (s *OpenAIGatewayService) lookupOrCreateCodexThreadID(ctx context.Context, account *Account, seed, threadInput string) string {
	if account == nil || seed == "" || strings.TrimSpace(threadInput) == "" {
		return ""
	}
	key := codexThreadIDStoreKey(account.ID, threadInput)
	fallback := func() string {
		return deriveCodexThreadUUIDv7(seed, codexThreadKindThread, threadInput, codexThreadFallbackUnixMs(seed, codexThreadKindThread, threadInput))
	}
	if s == nil {
		return fallback()
	}

	now := time.Now()
	if raw, ok := s.openaiCodexThreadIDs.Load(key); ok {
		if entry, ok := raw.(codexThreadIDL1Entry); ok && now.Before(entry.expiresAt) && entry.value != "" {
			return entry.value
		}
		s.openaiCodexThreadIDs.Delete(key)
	}

	store, _ := s.cache.(codexThreadIDStore)
	if store == nil {
		return fallback()
	}
	storeCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()

	if existing, err := store.GetCodexThreadID(storeCtx, key); err == nil && existing != "" {
		s.rememberCodexThreadID(key, existing, now)
		return existing
	} else if err != nil {
		return fallback()
	}

	created := deriveCodexThreadUUIDv7(seed, codexThreadKindThread, threadInput, now.UnixMilli())
	if err := store.SetCodexThreadID(storeCtx, key, created, codexThreadIDStoreTTL); err != nil {
		return fallback()
	}
	// SetNX 语义：并发首写时以 Redis 里落地的为准，读回一次保证多副本一致。
	if stored, err := store.GetCodexThreadID(storeCtx, key); err == nil && stored != "" {
		created = stored
	}
	s.rememberCodexThreadID(key, created, now)
	return created
}

func (s *OpenAIGatewayService) rememberCodexThreadID(key, value string, now time.Time) {
	s.openaiCodexThreadIDs.Store(key, codexThreadIDL1Entry{value: value, expiresAt: now.Add(codexThreadIDStoreTTL)})
	if s.openaiCodexThreadIDWrites.Add(1)%codexThreadIDL1SweepEvery != 0 {
		return
	}
	s.openaiCodexThreadIDs.Range(func(k, v any) bool {
		entry, ok := v.(codexThreadIDL1Entry)
		if !ok || now.After(entry.expiresAt) {
			s.openaiCodexThreadIDs.Delete(k)
		}
		return true
	})
}

// ---------------------------------------------------------------------------
// 解析器
// ---------------------------------------------------------------------------

// deriveCodexThreadKindID 按“原值是 v7 → 保时间戳派生；否则 → 走存储/回退”的统一规则派生。
func (s *OpenAIGatewayService) deriveCodexThreadKindID(ctx context.Context, account *Account, seed, kind, original string, useStore bool) (string, int64) {
	original = strings.TrimSpace(original)
	if original == "" {
		return "", 0
	}
	if parsed, ok := parseUUIDv7(original); ok {
		ms := uuidV7UnixMs(parsed)
		return deriveCodexThreadUUIDv7(seed, kind, original, ms), ms
	}
	if useStore && kind == codexThreadKindThread {
		derived := s.lookupOrCreateCodexThreadID(ctx, account, seed, original)
		if parsed, ok := parseUUIDv7(derived); ok {
			return derived, uuidV7UnixMs(parsed)
		}
	}
	ms := codexThreadFallbackUnixMs(seed, kind, original)
	return deriveCodexThreadUUIDv7(seed, kind, original, ms), ms
}

// codexThreadWindowNumber 解析客户端原 window id 的 ":<n>" 后缀；无/非法 → 0（CPA 默认）。
func codexThreadWindowNumber(windowID string) int {
	windowID = strings.TrimSpace(windowID)
	idx := strings.LastIndexByte(windowID, ':')
	if idx < 0 || idx == len(windowID)-1 {
		return 0
	}
	n, err := strconv.Atoi(windowID[idx+1:])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// buildCodexThreadTurnMetadata 构造 x-codex-turn-metadata：以客户端原 JSON 为底（非法/缺失则空），
// 写入抓包表的八个键并替换 parent_thread_id，其余键（sandbox / thread_source …）原样保留；
// 不注入 turn_started_at_unix_ms（抓包表没有，原有则保留）。
func buildCodexThreadTurnMetadata(originalRaw string, ids *codexFingerprintIDs) string {
	metadata := map[string]any{}
	if raw := strings.TrimSpace(originalRaw); raw != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed != nil {
			metadata = parsed
		}
	}
	metadata["session_id"] = ids.sessionID
	metadata["thread_id"] = ids.threadID
	metadata["turn_id"] = ids.turnID
	metadata["root_turn_id"] = ids.rootTurnID
	metadata["installation_id"] = ids.installationID
	metadata["window_id"] = ids.windowID
	metadata["window_number"] = ids.windowNumber
	metadata["context_window_id"] = ids.contextWindowID
	if _, ok := metadata["parent_thread_id"]; ok || ids.parentThreadID != "" {
		if ids.parentThreadID != "" {
			metadata["parent_thread_id"] = ids.parentThreadID
		} else {
			delete(metadata, "parent_thread_id")
		}
	}
	rebuilt, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(rebuilt)
}

// resolveCodexThreadFingerprintIDs thread 模式的 ID 集合解析。返回 nil 表示不改写
// （账号无 seed：与其它收敛模式一致，运维必须同时种 seed）。
func (s *OpenAIGatewayService) resolveCodexThreadFingerprintIDs(ctx context.Context, account *Account, o codexThreadOriginals) *codexFingerprintIDs {
	if account == nil {
		return nil
	}
	seed, ok := codexFingerprintSeed(account.Extra)
	if !ok {
		return nil
	}
	installationID := resolveConvergedInstallationID(account, seed)
	if installationID == "" {
		return nil
	}

	ids := &codexFingerprintIDs{
		accountID:                     account.ID,
		mode:                          codexFingerprintThread,
		installationID:                installationID,
		turnStartedAtUnixMs:           time.Now().UnixMilli(),
		originalBodySessionIDCaptured: true,
		originalBodySessionID:         o.sessionID,
	}

	// thread：prompt_cache_key → thread_id → 内容种子；v7 保时间戳，否则走存储保证跨请求稳定；
	// 完全无键时只能每次新 v7（无从稳定，文档已注明）。
	threadInput := o.threadInput()
	var threadMs int64
	if threadInput != "" {
		ids.threadID, threadMs = s.deriveCodexThreadKindID(ctx, account, seed, codexThreadKindThread, threadInput, true)
	}
	if ids.threadID == "" {
		fresh := uuid.Must(uuid.NewV7())
		ids.threadID, threadMs = fresh.String(), uuidV7UnixMs(fresh)
	}
	ids.sessionID = ids.threadID

	// turn：原 turn_id 有 → 派生保时间戳（重试同 turn 不变）；无 → 每次 Forward 一个新 v7。
	if o.turnID != "" {
		ids.turnID, _ = s.deriveCodexThreadKindID(ctx, account, seed, codexThreadKindTurn, o.turnID, false)
	}
	if ids.turnID == "" {
		ids.turnID = uuid.Must(uuid.NewV7()).String()
	}
	ids.rootTurnID = ids.turnID

	// context_window：随 thread 派生；原值是 v7 则沿用其时间戳。
	contextMs := threadMs
	if parsed, ok := parseUUIDv7(o.contextWindowID); ok {
		contextMs = uuidV7UnixMs(parsed)
	}
	contextInput := threadInput
	if contextInput == "" {
		contextInput = ids.threadID
	}
	ids.contextWindowID = deriveCodexThreadUUIDv7(seed, codexThreadKindContextWindow, contextInput, contextMs)

	// window：<thread>:<n>，n 保留客户端原后缀，无则 0。
	ids.windowNumber = codexThreadWindowNumber(o.windowID)
	ids.windowID = ids.threadID + ":" + strconv.Itoa(ids.windowNumber)

	// parent：原值存在才发，与主线程同规则派生（父会话自己作为主线程时得到同一值）。
	if o.parentThreadID != "" {
		ids.parentThreadID, _ = s.deriveCodexThreadKindID(ctx, account, seed, codexThreadKindThread, o.parentThreadID, true)
	}

	ids.turnMetadataJSON = buildCodexThreadTurnMetadata(o.turnMetadataRaw, ids)

	// 响应去混淆映射：只登记原值非空且确实改变了的对。
	if o.promptCacheKey != "" && o.promptCacheKey != ids.threadID {
		ids.exposeReplacements = append(ids.exposeReplacements, codexIdentityReplacement{derived: ids.threadID, original: o.promptCacheKey})
	} else if o.threadID != "" && o.threadID != ids.threadID {
		ids.exposeReplacements = append(ids.exposeReplacements, codexIdentityReplacement{derived: ids.threadID, original: o.threadID})
	}
	if o.turnID != "" && o.turnID != ids.turnID {
		ids.exposeReplacements = append(ids.exposeReplacements, codexIdentityReplacement{derived: ids.turnID, original: o.turnID})
	}
	return ids
}

// resolveCodexFingerprintIDsForAttempt 按账号模式分发：thread 走本文件，其余模式逐字节沿用
// resolveCodexFingerprintIDsFromRequest。
func (s *OpenAIGatewayService) resolveCodexFingerprintIDsForAttempt(ctx context.Context, account *Account, originals codexThreadOriginals, clientHeaders http.Header) *codexFingerprintIDs {
	if account != nil && account.GetCodexFingerprintMode() == codexFingerprintThread {
		return s.resolveCodexThreadFingerprintIDs(ctx, account, originals)
	}
	return resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
}

// ---------------------------------------------------------------------------
// 响应去混淆
// ---------------------------------------------------------------------------

// restoreCodexFingerprintIDsInPayload 把响应体里的派生值换回客户端原值（CPA
// applyCodexIdentityExposeResponsePayload 对等）：客户端应看到自己的 prompt_cache_key /
// turn_id。仅当前账号 staged 的 thread 模式 IDs 生效；派生值是完整 36 字符 UUID，
// 误替换概率可忽略，"<derived>:<n>" 形态的 window 因前缀匹配同样被还原。
func restoreCodexFingerprintIDsInPayload(c *gin.Context, account *Account, payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}
	ids := stagedCodexFingerprintIDs(c, account)
	if ids == nil || ids.mode != codexFingerprintThread || len(ids.exposeReplacements) == 0 {
		return payload
	}
	for _, pair := range ids.exposeReplacements {
		if pair.derived == "" || pair.original == "" || pair.derived == pair.original {
			continue
		}
		if !bytes.Contains(payload, []byte(pair.derived)) {
			continue
		}
		payload = bytes.ReplaceAll(payload, []byte(pair.derived), []byte(pair.original))
	}
	return payload
}

// ---------------------------------------------------------------------------
// 调试行
// ---------------------------------------------------------------------------

// logCodexIdentityDebug 打印出站身份摘要（SUB2API_DEBUG_CODEX_IDENTITY=1 时）。
func logCodexIdentityDebug(where string, account *Account, h http.Header, body []byte) {
	if !debugCodexIdentity || h == nil {
		return
	}
	keys := []string{
		"user-agent", "originator", "version",
		"session-id", "thread-id", "x-client-request-id", codexParentThreadIDHeader,
		"x-codex-installation-id", "x-codex-window-id", "x-codex-turn-metadata",
		"session_id", "conversation_id", "x-codex-beta-features", "x-codex-routing-hint",
	}
	parts := make([]string, 0, len(keys)+3)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, getHeaderRaw(h, k)))
	}
	if len(body) > 0 {
		parts = append(parts,
			fmt.Sprintf("body.prompt_cache_key=%q", gjson.GetBytes(body, "prompt_cache_key").String()),
			fmt.Sprintf("body.client_metadata=%s", gjson.GetBytes(body, "client_metadata").Raw),
		)
	}
	aid, mode := int64(0), codexFingerprintOff
	if account != nil {
		aid, mode = account.ID, account.GetCodexFingerprintMode()
	}
	logger.LegacyPrintf("service.openai_gateway", "[CodexIdentityDebug] where=%s account=%d mode=%s %s",
		where, aid, mode, strings.Join(parts, " "))
}
