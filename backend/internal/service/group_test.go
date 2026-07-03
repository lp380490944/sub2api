//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGroup_GetImagePrice_1K 测试 1K 尺寸返回正确价格
func TestGroup_GetImagePrice_1K(t *testing.T) {
	price := 0.10
	group := &Group{
		ImagePrice1K: &price,
	}

	result := group.GetImagePrice("1K")
	require.NotNil(t, result)
	require.InDelta(t, 0.10, *result, 0.0001)
}

// TestGroup_GetImagePrice_2K 测试 2K 尺寸返回正确价格
func TestGroup_GetImagePrice_2K(t *testing.T) {
	price := 0.15
	group := &Group{
		ImagePrice2K: &price,
	}

	result := group.GetImagePrice("2K")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)
}

// TestGroup_GetImagePrice_4K 测试 4K 尺寸返回正确价格
func TestGroup_GetImagePrice_4K(t *testing.T) {
	price := 0.30
	group := &Group{
		ImagePrice4K: &price,
	}

	result := group.GetImagePrice("4K")
	require.NotNil(t, result)
	require.InDelta(t, 0.30, *result, 0.0001)
}

// TestGroup_GetImagePrice_UnknownSize 测试未知尺寸回退 2K
func TestGroup_GetImagePrice_UnknownSize(t *testing.T) {
	price2K := 0.15
	group := &Group{
		ImagePrice2K: &price2K,
	}

	// 未知尺寸 "3K" 应该回退到 2K
	result := group.GetImagePrice("3K")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)

	// 空字符串也回退到 2K
	result = group.GetImagePrice("")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)
}

// TestGroup_GetImagePrice_NilValues 测试未配置时返回 nil
func TestGroup_GetImagePrice_NilValues(t *testing.T) {
	group := &Group{
		// 所有 ImagePrice 字段都是 nil
	}

	require.Nil(t, group.GetImagePrice("1K"))
	require.Nil(t, group.GetImagePrice("2K"))
	require.Nil(t, group.GetImagePrice("4K"))
	require.Nil(t, group.GetImagePrice("unknown"))
}

// TestGroup_GetImagePrice_PartialConfig 测试部分配置
func TestGroup_GetImagePrice_PartialConfig(t *testing.T) {
	price1K := 0.10
	group := &Group{
		ImagePrice1K: &price1K,
		// ImagePrice2K 和 ImagePrice4K 未配置
	}

	result := group.GetImagePrice("1K")
	require.NotNil(t, result)
	require.InDelta(t, 0.10, *result, 0.0001)

	// 2K 和 4K 返回 nil
	require.Nil(t, group.GetImagePrice("2K"))
	require.Nil(t, group.GetImagePrice("4K"))
}

// TestGroup_GetRoutingAccountIDsTiered_SpecificPlusWildcard 验证命中具体模型规则时，
// "*" 兜底规则被追加为更低梯队（梯队 1），具体规则账号为梯队 0。
func TestGroup_GetRoutingAccountIDsTiered_SpecificPlusWildcard(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"claude-opus-4-7": {10, 11, 12},
			"*":               {20, 21},
		},
	}

	ids, rank := group.GetRoutingAccountIDsTiered("claude-opus-4-7")

	require.Equal(t, []int64{10, 11, 12, 20, 21}, ids)
	require.Equal(t, map[int64]int{10: 0, 11: 0, 12: 0, 20: 1, 21: 1}, rank)

	// 薄封装 GetRoutingAccountIDs 返回扁平列表（含 * 梯队）
	require.Equal(t, []int64{10, 11, 12, 20, 21}, group.GetRoutingAccountIDs("claude-opus-4-7"))
}

// TestGroup_GetRoutingAccountIDsTiered_WildcardOnly 验证请求只命中 "*" 时不产生重复梯队，
// 全部为梯队 0。
func TestGroup_GetRoutingAccountIDsTiered_WildcardOnly(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"claude-opus-4-7": {10, 11},
			"*":               {20, 21},
		},
	}

	ids, rank := group.GetRoutingAccountIDsTiered("claude-sonnet-4-5")

	require.Equal(t, []int64{20, 21}, ids)
	require.Equal(t, map[int64]int{20: 0, 21: 0}, rank)
}

// TestGroup_GetRoutingAccountIDsTiered_DedupKeepsTier0 验证同时出现在具体规则与 "*" 规则的账号
// 保留在更高优先的梯队 0，且不在结果中重复。
func TestGroup_GetRoutingAccountIDsTiered_DedupKeepsTier0(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"claude-opus-4-7": {10, 11},
			"*":               {11, 20},
		},
	}

	ids, rank := group.GetRoutingAccountIDsTiered("claude-opus-4-7")

	require.Equal(t, []int64{10, 11, 20}, ids)
	require.Equal(t, map[int64]int{10: 0, 11: 0, 20: 1}, rank)
}

// TestGroup_GetRoutingAccountIDsTiered_LongestPrefixWins 验证多个通配规则时取最长前缀（最具体），
// 且 "*" 作为兜底梯队追加。
func TestGroup_GetRoutingAccountIDsTiered_LongestPrefixWins(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"claude-opus-*": {10, 11},
			"*":             {20},
		},
	}

	ids, rank := group.GetRoutingAccountIDsTiered("claude-opus-4-7")

	require.Equal(t, []int64{10, 11, 20}, ids)
	require.Equal(t, map[int64]int{10: 0, 11: 0, 20: 1}, rank)
}

// TestGroup_GetRoutingAccountIDsTiered_Disabled 验证路由未启用/无匹配时返回 nil。
func TestGroup_GetRoutingAccountIDsTiered_Disabled(t *testing.T) {
	group := &Group{
		ModelRoutingEnabled: false,
		ModelRouting:        map[string][]int64{"claude-opus-4-7": {10}},
	}
	ids, rank := group.GetRoutingAccountIDsTiered("claude-opus-4-7")
	require.Nil(t, ids)
	require.Nil(t, rank)

	// 启用但无匹配（也无 * 规则）
	group2 := &Group{
		ModelRoutingEnabled: true,
		ModelRouting:        map[string][]int64{"claude-opus-4-7": {10}},
	}
	ids2, rank2 := group2.GetRoutingAccountIDsTiered("gpt-4o")
	require.Nil(t, ids2)
	require.Nil(t, rank2)
}
