package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// CLIVersionTrackerService 周期性追随 Claude Code CLI 在 npm 上的最新版本号 (P1-2)。
//
// 行为：
//   - Start() 时先从 system_settings 回填 cli_current_version → claude.SetCLICurrentVersion；
//     之后每 IntervalHours 小时拉取一次 npm dist-tags.latest，去重后写回 settings 并刷新
//     claude 包的运行时版本变量。
//   - 维护 cli_recent_versions（JSON 数组，最新在前），最多 MaxRecentVersions 项；
//     PickVersionForAccount 用此列表做 75/20/5 的版本扰动。
//   - npm 拉取失败不影响启动，只记录日志，下个周期重试。
type CLIVersionTrackerService struct {
	settingRepo SettingRepository
	cfg         config.CLIVersionTrackerConfig
	httpClient  *http.Client

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewCLIVersionTrackerService 构造一个未启动的 tracker；调用方需 Start()。
func NewCLIVersionTrackerService(settingRepo SettingRepository, cfg config.CLIVersionTrackerConfig) *CLIVersionTrackerService {
	timeout := time.Duration(cfg.RequestTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &CLIVersionTrackerService{
		settingRepo: settingRepo,
		cfg:         cfg,
		httpClient:  &http.Client{Timeout: timeout},
		stopCh:      make(chan struct{}),
	}
}

// Start 回填 DB → 启动周期 ticker（如果 Enabled）。
func (s *CLIVersionTrackerService) Start() {
	if s == nil || s.settingRepo == nil {
		return
	}

	// 启动时先 reload 一次：让运行时版本号与 DB 对齐。
	if err := s.ReloadFromDB(context.Background()); err != nil {
		log.Printf("[CLIVersionTracker] reload from DB failed: %v", err)
	}

	if !s.cfg.Enabled || s.cfg.IntervalHours <= 0 {
		log.Printf("[CLIVersionTracker] periodic fetch disabled (enabled=%v, interval_hours=%d); using static value=%q",
			s.cfg.Enabled, s.cfg.IntervalHours, claude.GetCLICurrentVersion())
		return
	}

	interval := time.Duration(s.cfg.IntervalHours) * time.Hour
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 启动时先做一次拉取，避免冷启动后等到第一个 tick
		s.runOnce(context.Background())
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runOnce(context.Background())
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 关闭后台 goroutine 并等待退出。
func (s *CLIVersionTrackerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

// ReloadFromDB 从 system_settings 读取当前版本与 recent 列表，回填到 claude 包运行时变量。
// 不写 settings；纯单向 DB → 运行时同步。
func (s *CLIVersionTrackerService) ReloadFromDB(ctx context.Context) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	values, err := s.settingRepo.GetMultiple(dbCtx, []string{
		SettingKeyCLICurrentVersion,
		SettingKeyCLIRecentVersions,
	})
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	current := strings.TrimSpace(values[SettingKeyCLICurrentVersion])
	if current != "" {
		if !claude.SetCLICurrentVersion(current) {
			log.Printf("[CLIVersionTracker] DB cli_current_version=%q invalid (not semver); ignoring", current)
		} else {
			log.Printf("[CLIVersionTracker] reload: cli_current_version=%q from DB", current)
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyCLIRecentVersions]); raw != "" {
		var recent []string
		if err := json.Unmarshal([]byte(raw), &recent); err == nil {
			SetCachedRecentVersions(recent)
			log.Printf("[CLIVersionTracker] reload: cli_recent_versions=%v from DB", recent)
		}
	}
	return nil
}

// runOnce 执行一次"拉取 → 比较 → 写回 → 刷新运行时"。失败时只记日志。
func (s *CLIVersionTrackerService) runOnce(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	latest, err := s.fetchNpmLatest(ctx)
	if err != nil {
		log.Printf("[CLIVersionTracker] fetch npm failed: %v", err)
		return
	}
	if !semverStrictRe.MatchString(latest) {
		log.Printf("[CLIVersionTracker] npm returned non-semver %q; skip", latest)
		return
	}

	current := claude.GetCLICurrentVersion()
	if latest == current {
		// 仍可能需要回填 settings（首次启动），但不写运行时
		return
	}

	// 读 recent，把新版本 push 到头部，保留 max 个
	dbCtx, dbCancel := context.WithTimeout(ctx, 5*time.Second)
	defer dbCancel()
	values, err := s.settingRepo.GetMultiple(dbCtx, []string{SettingKeyCLIRecentVersions})
	if err != nil {
		log.Printf("[CLIVersionTracker] read cli_recent_versions failed: %v", err)
		return
	}
	var recent []string
	if raw := strings.TrimSpace(values[SettingKeyCLIRecentVersions]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &recent)
	}
	recent = pushVersionFront(recent, latest, s.maxRecent())

	encoded, err := json.Marshal(recent)
	if err != nil {
		log.Printf("[CLIVersionTracker] marshal recent failed: %v", err)
		return
	}
	if err := s.settingRepo.SetMultiple(dbCtx, map[string]string{
		SettingKeyCLICurrentVersion: latest,
		SettingKeyCLIRecentVersions: string(encoded),
	}); err != nil {
		log.Printf("[CLIVersionTracker] write settings failed: %v", err)
		return
	}

	if !claude.SetCLICurrentVersion(latest) {
		log.Printf("[CLIVersionTracker] refused to apply invalid version %q", latest)
		return
	}
	SetCachedRecentVersions(recent)
	log.Printf("[CLIVersionTracker] upgraded cli_current_version: %q → %q (recent=%v)", current, latest, recent)
}

func (s *CLIVersionTrackerService) maxRecent() int {
	if s.cfg.MaxRecentVersions > 0 {
		return s.cfg.MaxRecentVersions
	}
	return 3
}

// fetchNpmLatest 从 npm dist-tags 端点获取 latest 版本号。
// 响应示例：{"latest":"2.1.117","beta":"2.2.0-beta.1"}
func (s *CLIVersionTrackerService) fetchNpmLatest(ctx context.Context) (string, error) {
	url := strings.TrimSpace(s.cfg.NpmRegistryURL)
	if url == "" {
		url = "https://registry.npmjs.org/-/package/@anthropic-ai/claude-code/dist-tags"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-cli-version-tracker/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var tags struct {
		Latest string `json:"latest"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return strings.TrimSpace(tags.Latest), nil
}

// pushVersionFront 把 v 推到 list 头部，去除已存在的同值，截断到 max 项。
func pushVersionFront(list []string, v string, max int) []string {
	if max <= 0 {
		max = 3
	}
	out := make([]string, 0, max)
	out = append(out, v)
	for _, x := range list {
		if x == v {
			continue
		}
		out = append(out, x)
		if len(out) >= max {
			break
		}
	}
	return out
}

// semverStrictRe 严格 X.Y.Z 三段 semver。
var semverStrictRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// 进程内 cached recent versions（用于 PickVersionForAccount 热路径）。
// 由 ReloadFromDB / runOnce 写入；读取通过 GetCachedRecentVersions。
var (
	cachedRecentVersions []string
	cachedRecentMu       sync.RWMutex
)

// SetCachedRecentVersions 替换进程内缓存的 recent 版本列表。
func SetCachedRecentVersions(list []string) {
	cachedRecentMu.Lock()
	defer cachedRecentMu.Unlock()
	cachedRecentVersions = append([]string(nil), list...)
}

// GetCachedRecentVersions 返回进程内缓存的 recent 版本列表副本。
func GetCachedRecentVersions() []string {
	cachedRecentMu.RLock()
	defer cachedRecentMu.RUnlock()
	if len(cachedRecentVersions) == 0 {
		return nil
	}
	return append([]string(nil), cachedRecentVersions...)
}

// PickCLIVersionForAccount 是 claude.PickVersionForAccount 的进程内缓存包装。
func PickCLIVersionForAccount(accountID int64) string {
	recent := GetCachedRecentVersions()
	return claude.PickVersionForAccount(accountID, recent)
}

var _ = logger.LegacyPrintf // avoid unused import when log is preferred above
