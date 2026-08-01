package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newGuardRouter 构造最小中间件链：注入指定 role，再挂 ReadonlyAdminGuard。
func newGuardRouter(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUserRole), role)
		c.Next()
	})
	r.Use(ReadonlyAdminGuard())
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	// 复用真实白名单里的路径模板，确保 FullPath 归一化行为被真实覆盖。
	r.GET("/api/v1/admin/accounts", ok)
	r.GET("/api/v1/admin/accounts/:id", ok)
	r.POST("/api/v1/admin/accounts", ok)
	r.GET("/api/v1/admin/accounts/data", ok)
	r.DELETE("/api/v1/admin/accounts/:id", ok)
	return r
}

func doGuardRequest(r *gin.Engine, method, path string) int {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w.Code
}

func TestReadonlyAdminGuardPassesThroughForAdmin(t *testing.T) {
	r := newGuardRouter(service.RoleAdmin)
	// admin 不受影响：读写都放行，中间件必须零影响。
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts"))
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodPost, "/api/v1/admin/accounts"))
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodDelete, "/api/v1/admin/accounts/7"))
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts/data"))
}

func TestReadonlyAdminGuardAllowsWhitelistedReads(t *testing.T) {
	r := newGuardRouter(service.RoleReadonlyAdmin)
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts"))
	// 路径参数必须经 FullPath 归一化到同一条白名单项，不同 id 行为一致。
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts/7"))
	require.Equal(t, http.StatusOK, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts/99999"))
}

func TestReadonlyAdminGuardBlocksWrites(t *testing.T) {
	r := newGuardRouter(service.RoleReadonlyAdmin)
	require.Equal(t, http.StatusForbidden, doGuardRequest(r, http.MethodPost, "/api/v1/admin/accounts"))
	require.Equal(t, http.StatusForbidden, doGuardRequest(r, http.MethodDelete, "/api/v1/admin/accounts/7"))
}

// GET /accounts/data 导出上游凭据原文。它是 GET，但绝不能因此被放行。
func TestReadonlyAdminGuardBlocksDangerousGET(t *testing.T) {
	r := newGuardRouter(service.RoleReadonlyAdmin)
	require.Equal(t, http.StatusForbidden, doGuardRequest(r, http.MethodGet, "/api/v1/admin/accounts/data"))
}
