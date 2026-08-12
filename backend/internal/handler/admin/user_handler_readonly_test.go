package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newStubAdminServiceCapturingFilters returns a stub whose ListUsers records
// the filters it was called with into stubAdminService.lastListUsers.filters
// (an existing field on the shared stub), and whose sole user has ID selfID.
// The captured pointer mirrors that field after each List() call so callers
// can assert on it directly.
func newStubAdminServiceCapturingFilters(selfID int64) *stubAdminService {
	stub := newStubAdminService()
	stub.users = []service.User{{
		ID:     selfID,
		Email:  "self@example.com",
		Role:   service.RoleReadonlyAdmin,
		Status: service.StatusActive,
	}}
	return stub
}

// newStubAdminServiceReturningUser returns a stub with a user at ID 42
// (matching selfID used across the GetByID tests) and another at ID 99.
func newStubAdminServiceReturningUser() *stubAdminService {
	stub := newStubAdminService()
	stub.users = []service.User{
		{ID: 42, Email: "self@example.com", Role: service.RoleReadonlyAdmin, Status: service.StatusActive},
		{ID: 99, Email: "other@example.com", Role: service.RoleUser, Status: service.StatusActive},
	}
	return stub
}

// newUserHandlerForTest builds a UserHandler wired only with the adminService,
// mirroring the construction used elsewhere in this package's tests.
func newUserHandlerForTest(stub *stubAdminService) *UserHandler {
	return NewUserHandler(stub, nil, nil, nil, nil, nil, nil)
}

// newAdminTestContext 构造带 role 与 UserID 的 gin 测试上下文，
// 模拟 adminAuth 中间件写入 context 的结果。
func newAdminTestContext(method, target string, userID int64, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	c.Set(string(middleware2.ContextKeyUserRole), role)
	return c, w
}

// readonly_admin 调 List 时，无论请求带什么查询参数，
// 传给 adminService.ListUsers 的 filters.RestrictToUserID 都必须等于自身 ID。
func TestUserHandlerListRestrictsReadonlyAdminToSelf(t *testing.T) {
	const selfID int64 = 42
	stub := newStubAdminServiceCapturingFilters(selfID)
	h := newUserHandlerForTest(stub)

	for _, query := range []string{
		"",
		"?search=other",
		"?status=active&role=admin",
		"?group_name=vip&api_key_group_id=3",
	} {
		c, w := newAdminTestContext(http.MethodGet, "/api/v1/admin/users"+query, selfID, service.RoleReadonlyAdmin)
		h.List(c)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equalf(t, selfID, stub.lastListUsers.filters.RestrictToUserID,
			"query %q must not widen the readonly_admin scope", query)
	}
}

// admin 不受影响：RestrictToUserID 必须为 0。
func TestUserHandlerListDoesNotRestrictAdmin(t *testing.T) {
	stub := newStubAdminService()
	h := newUserHandlerForTest(stub)
	c, w := newAdminTestContext(http.MethodGet, "/api/v1/admin/users", 1, service.RoleAdmin)
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Zero(t, stub.lastListUsers.filters.RestrictToUserID)
}

// readonly_admin 查他人详情返回 404（不是 403 —— 403 会泄露该 ID 存在）。
func TestUserHandlerGetByIDHidesOtherUsersFromReadonlyAdmin(t *testing.T) {
	const selfID int64 = 42
	h := newUserHandlerForTest(newStubAdminServiceReturningUser())
	c, w := newAdminTestContext(http.MethodGet, "/api/v1/admin/users/99", selfID, service.RoleReadonlyAdmin)
	c.Params = gin.Params{{Key: "id", Value: "99"}}
	h.GetByID(c)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// readonly_admin 查自己详情正常返回。
func TestUserHandlerGetByIDAllowsSelfForReadonlyAdmin(t *testing.T) {
	const selfID int64 = 42
	h := newUserHandlerForTest(newStubAdminServiceReturningUser())
	c, w := newAdminTestContext(http.MethodGet, "/api/v1/admin/users/42", selfID, service.RoleReadonlyAdmin)
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	h.GetByID(c)
	require.Equal(t, http.StatusOK, w.Code)
}
