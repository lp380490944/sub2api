package claude

import (
	"regexp"
	"strings"
	"sync"
)

// fork(P1-2)：Claude Code CLI 版本号的运行时可变副本。
//
// 上游把版本号写死在 CLIDefaultVersion，只随发版更新，实际会落后真实 CLI 数十个小版本。
// CLIVersionTrackerService 会周期性从 npm 拉取最新版本并通过 SetCLICurrentVersion 刷新，
// 同时同步改写 DefaultHeaders["User-Agent"]，保证 UA 与 cc_version 的一致性不变量。

var (
	cliCurrentVersion string
	cliVersionMu      sync.RWMutex

	// uaVersionRewriteRe 用于在 DefaultHeaders["User-Agent"] 中替换版本号片段。
	uaVersionRewriteRe = regexp.MustCompile(`claude-cli/\d+\.\d+\.\d+`)
	// semverRe 严格 X.Y.Z 三段 semver。
	semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// CLICurrentVersion 返回当前运行时版本；保留函数形态以兼容老调用方。
//
// Deprecated: use GetCLICurrentVersion() instead.
func CLICurrentVersion() string { //nolint:revive // legacy name kept for compatibility
	return GetCLICurrentVersion()
}

// GetCLICurrentVersion 返回当前运行时的 CLI 版本号（线程安全）。
func GetCLICurrentVersion() string {
	cliVersionMu.RLock()
	defer cliVersionMu.RUnlock()
	if cliCurrentVersion == "" {
		return CLIDefaultVersion
	}
	return cliCurrentVersion
}

// SetCLICurrentVersion 更新运行时 CLI 版本号；同步刷新 DefaultHeaders["User-Agent"]。
// 传入空字符串视为重置为 CLIDefaultVersion。
//
// 仅在严格 semver `X.Y.Z` 格式时接受，否则返回 false 并保持原值（防止 npm 偶发返回
// pre-release / 错误格式时污染 UA）。
func SetCLICurrentVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		v = CLIDefaultVersion
	}
	if !semverRe.MatchString(v) {
		return false
	}
	cliVersionMu.Lock()
	defer cliVersionMu.Unlock()
	cliCurrentVersion = v
	if ua, ok := DefaultHeaders["User-Agent"]; ok {
		DefaultHeaders["User-Agent"] = uaVersionRewriteRe.ReplaceAllString(ua, "claude-cli/"+v)
	}
	return true
}
