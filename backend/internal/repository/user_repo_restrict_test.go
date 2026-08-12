package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestListWithFiltersRestrictToUserID 验证 RestrictToUserID 是不可绕过的安全边界。
func TestListWithFiltersRestrictToUserID(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	ctx := context.Background()

	self := &service.User{
		Email:    "self@example.com",
		Username: "self",
		// Notes 故意包含 "other"：确保下方 Search:"other" 这一用例本身也会
		// 命中 self（否则该用例只是"两边都不匹配"的空集，无法证明"即使匹配
		// 到 self 之外的过滤条件，也拿不到 self 之外的数据"这一点）。
		Notes:        "other-note",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, self))

	other1 := &service.User{
		Email:        "other1@example.com",
		Username:     "other1",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, other1))

	other2 := &service.User{
		Email:        "other2@example.com",
		Username:     "other2",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, other2))

	selfID := self.ID
	params := pagination.PaginationParams{Page: 1, PageSize: 50, SortBy: "email", SortOrder: "asc"}

	t.Run("returns only the restricted user", func(t *testing.T) {
		users, _, err := repo.ListWithFilters(ctx, params, service.UserListFilters{
			RestrictToUserID: selfID,
		})
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, selfID, users[0].ID)
	})

	t.Run("cannot be widened by other filters", func(t *testing.T) {
		// 即使同时传入会命中其他用户的 Search/Status，也只能拿到自己那条。
		for _, f := range []service.UserListFilters{
			{RestrictToUserID: selfID, Search: "other"},
			{RestrictToUserID: selfID, Search: "@example.com"},
			{RestrictToUserID: selfID, Status: "active"},
		} {
			users, _, err := repo.ListWithFilters(ctx, params, f)
			require.NoError(t, err)
			require.Len(t, users, 1)
			require.Equal(t, selfID, users[0].ID)
		}
	})

	t.Run("zero value disables the restriction", func(t *testing.T) {
		users, _, err := repo.ListWithFilters(ctx, params, service.UserListFilters{})
		require.NoError(t, err)
		require.Len(t, users, 3)
	})
}
