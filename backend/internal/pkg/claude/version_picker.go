package claude

import (
	"fmt"
	"hash/fnv"
)

// PickVersionForAccount 根据 accountID 稳定地从 recent 列表中挑选一个 CLI 版本。
//
// 分桶策略 (P1-2)：
//   - 75% 账号 → recent[0]（latest）
//   - 20% 账号 → recent[1]（N-1）
//   - 5%  账号 → recent[2]（N-2）
//
// 边界处理：
//   - len(recent) == 0：返回 GetCLICurrentVersion() 兜底
//   - len(recent) == 1：所有账号都用 recent[0]
//   - len(recent) == 2：5% 桶降级到 recent[1]（无 N-2 可用）
//   - accountID <= 0（如未注册客户端）：返回 recent[0]，避免给"虚拟账号"分配低版本
//
// hash 选用 fnv32a 确保分布稳定，且无需依赖 crypto；同一 accountID 在多次调用、
// 多次重启间的输出严格一致（recent 不变时）。recent 内容变化会导致部分账号迁移
// 到不同版本，这是预期行为（latest 升级会让某些账号新被分配 N-2，是正常的扰动）。
func PickVersionForAccount(accountID int64, recent []string) string {
	if len(recent) == 0 {
		return GetCLICurrentVersion()
	}
	if len(recent) == 1 || accountID <= 0 {
		return recent[0]
	}
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "cli-version:%d", accountID)
	bucket := h.Sum32() % 100
	switch {
	case bucket < 5 && len(recent) >= 3:
		return recent[2]
	case bucket < 25 && len(recent) >= 2:
		return recent[1]
	default:
		return recent[0]
	}
}

// BuildUserAgentForVersion 根据版本号生成 claude-cli UA 字符串。
// 与 DefaultHeaders["User-Agent"] 同构，仅替换版本号。
func BuildUserAgentForVersion(version string) string {
	if version == "" {
		version = GetCLICurrentVersion()
	}
	return fmt.Sprintf("claude-cli/%s (external, cli)", version)
}
