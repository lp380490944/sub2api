package claude

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/mod/semver"
)

// CLIVersionEnv 是 CLICurrentVersion 的可选运维覆盖。
//
// 存在的理由：Anthropic 会对新模型设客户端版本下限（例如 claude-fable-5-1 要求
// claude-cli >= 2.1.251），命中时上游直接返回
// `Claude Code X.Y.Z does not support this model; version A.B.C or newer is required`。
// 在没有本开关之前，这类模型必须等 sub2api 发一个新版本才能使用，
// 而改动本身只是一个常量。xai 包的 XAI_GROK_CLI_VERSION 已经是同样的做法。
const CLIVersionEnv = "SUB2API_CLAUDE_CLI_VERSION"

// resolvedCLIVersion 在包初始化时解析一次：内置基线叠加环境变量覆盖。
//
// 它是运行时版本的"静态下限"：CLIVersionTrackerService 从 npm 拉到的更新版本可以
// 覆盖它（见 SetCLICurrentVersion），但任何低于它的值都会被拒绝。
var resolvedCLIVersion = resolveCLIVersion(os.Getenv(CLIVersionEnv))

// fork(P1-2)：Claude Code CLI 版本号的运行时可变副本。
//
// 上游把版本号写死在 CLICurrentVersion，只随发版更新，实际会落后真实 CLI 数十个小版本。
// CLIVersionTrackerService 会周期性从 npm 拉取最新版本并通过 SetCLICurrentVersion 刷新，
// 同时同步改写 DefaultHeaders["User-Agent"]，保证 UA 与 cc_version 的一致性不变量。
var (
	cliCurrentVersion string
	cliVersionMu      sync.RWMutex

	// uaVersionRewriteRe 用于在 DefaultHeaders["User-Agent"] 中替换版本号片段。
	uaVersionRewriteRe = regexp.MustCompile(`claude-cli/\d+\.\d+\.\d+`)
)

// CLIVersion 返回对外伪装的 Claude Code CLI 版本号（三段 semver）。
//
// 所有需要该版本号的位置都必须走本函数，不要直接引用 CLICurrentVersion——
// 后者只是"没有覆盖时的内置基线"。取值优先级：
// npm tracker 运行时值（SetCLICurrentVersion）> SUB2API_CLAUDE_CLI_VERSION > CLICurrentVersion。
func CLIVersion() string {
	cliVersionMu.RLock()
	defer cliVersionMu.RUnlock()
	if cliCurrentVersion == "" {
		return resolvedCLIVersion
	}
	return cliCurrentVersion
}

// GetCLICurrentVersion 与 CLIVersion 等价，保留 fork 侧的调用名。
func GetCLICurrentVersion() string {
	return CLIVersion()
}

// SetCLICurrentVersion 更新运行时 CLI 版本号；同步刷新 DefaultHeaders["User-Agent"]。
// 传入空字符串视为重置为静态下限（内置基线/环境变量覆盖）。
//
// 只接受严格 `X.Y.Z` 且不低于静态下限的版本，否则返回 false 并保持原值：
// 防止 npm 偶发返回 pre-release、或 DB 里残留的过旧值把 UA 拉回到新模型闸门之下。
func SetCLICurrentVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		v = resolvedCLIVersion
	}
	if !IsSupportedCLIVersion(v) {
		return false
	}
	cliVersionMu.Lock()
	defer cliVersionMu.Unlock()
	if semver.Compare("v"+v, "v"+resolvedCLIVersion) < 0 {
		return false
	}
	cliCurrentVersion = v
	if ua, ok := DefaultHeaders["User-Agent"]; ok {
		DefaultHeaders["User-Agent"] = uaVersionRewriteRe.ReplaceAllString(ua, "claude-cli/"+v)
	}
	return true
}

// IsSupportedCLIVersion 判断运维给的覆盖值是否可用。
//
// 判据有两条，缺一不可：
//  1. 严格三段纯数字（"2.1.251"）。带 -local / -dev / +build 等后缀的版本号会被
//     identity_service 的 fingerprintUserAgentPattern 拒绝，一旦漏进去，该账号的
//     持久指纹会被写成一个不存在的客户端版本，此后所有上游请求都声称这个版本，
//     被判非正版并持续 429——而系统内没有指纹重置入口。
//  2. 不低于内置基线 CLICurrentVersion。向下覆盖没有任何使用场景，
//     却会让 identity_service 的主版本超前检查基准跟着一起降。
func IsSupportedCLIVersion(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}
	// semver 允许 "v1.2" 与预发布/构建元数据，这里都不接受：
	// Canonical 相等可排除省略段，再显式排除预发布与构建元数据。
	canonical := "v" + version
	if !semver.IsValid(canonical) || semver.Canonical(canonical) != canonical {
		return false
	}
	if semver.Prerelease(canonical) != "" || semver.Build(canonical) != "" {
		return false
	}
	return semver.Compare(canonical, "v"+CLICurrentVersion) >= 0
}

// resolveCLIVersion 把环境变量的原始值解析成可用的版本号。
// 空值静默回落（未配置是正常状态）；非空但不合法则回落并告警——
// 静默忽略一个显式配置会让运维以为已经生效。
func resolveCLIVersion(raw string) string {
	version := strings.TrimSpace(raw)
	if version == "" {
		return CLICurrentVersion
	}
	if !IsSupportedCLIVersion(version) {
		slog.Warn("ignoring invalid Claude CLI version override; falling back to the built-in pin",
			"env", CLIVersionEnv,
			"value", version,
			"builtin", CLICurrentVersion,
			"requirement", "strict three-part semver, not older than the built-in pin")
		return CLICurrentVersion
	}
	return version
}
