package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// noComplianceAckRepoStub is a SettingRepository that never has an
// acknowledgement stored, i.e. admin compliance is permanently unacknowledged.
type noComplianceAckRepoStub struct{}

func (noComplianceAckRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (noComplianceAckRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (noComplianceAckRepoStub) Set(ctx context.Context, key, value string) error { return nil }

func (noComplianceAckRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (noComplianceAckRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (noComplianceAckRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (noComplianceAckRepoStub) Delete(ctx context.Context, key string) error { return nil }

// newAdminRouterWithRoleAndCompliance mirrors newAdminRouterWithRole but wires a
// *real* SettingService (backed by a stub repo that reports compliance as
// permanently unacknowledged) instead of nil, so AdminComplianceGuard actually
// runs instead of short-circuiting on a nil settingService. This is what lets
// this test exercise the real, fully mounted middleware chain
// (adminAuth -> auditLog -> AdminComplianceGuard -> ReadonlyAdminGuard) rather
// than the guard functions in isolation.
func newAdminRouterWithRoleAndCompliance(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.AbortWithStatus(599) // reached the (nil) handler stub
			}
		}()
		c.Next()
	})
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	settingService := service.NewSettingService(noComplianceAckRepoStub{}, &config.Config{})
	setSubjectAndRole := func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Set(string(servermiddleware.ContextKeyUserRole), role)
		c.Next()
	}
	noop := func(c *gin.Context) { c.Next() }
	v1 := router.Group("/api/v1")
	RegisterAdminRoutes(
		v1,
		handlers,
		servermiddleware.AdminAuthMiddleware(setSubjectAndRole),
		servermiddleware.AuditLogMiddleware(noop),
		servermiddleware.StepUpAuthMiddleware(noop),
		settingService,
		nil,
	)
	return router
}

// TestAdminComplianceGuardDoesNotBlockReadonlyAdminOnRealRouteChain reproduces
// the exact bug from the final review: a newly created readonly_admin got 423
// ADMIN_COMPLIANCE_ACK_REQUIRED on every allowlisted endpoint, with no escape
// hatch, because AdminComplianceGuard ran unconditionally ahead of
// ReadonlyAdminGuard and /compliance* is itself denied by the readonly
// allowlist.
//
// This goes through the real mounted chain built by RegisterAdminRoutes
// (adminAuth -> auditLog -> AdminComplianceGuard -> ReadonlyAdminGuard) with a
// real SettingService reporting "unacknowledged" — an isolated test of
// AdminComplianceGuard alone would not have caught this, since the bug is
// about *ordering* relative to ReadonlyAdminGuard and the deny-by-default
// allowlist, not about the guard's own logic.
func TestAdminComplianceGuardDoesNotBlockReadonlyAdminOnRealRouteChain(t *testing.T) {
	readonly := newAdminRouterWithRoleAndCompliance(service.RoleReadonlyAdmin)

	w := httptest.NewRecorder()
	readonly.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil))
	require.NotEqual(t, http.StatusLocked, w.Code,
		"readonly_admin got 423 ADMIN_COMPLIANCE_ACK_REQUIRED on an allowlisted endpoint with no way to "+
			"acknowledge (compliance endpoints are themselves denied by the readonly allowlist)")
	require.NotEqual(t, http.StatusForbidden, w.Code,
		"GET /admin/accounts is allowlisted for readonly_admin and must not be blocked by ReadonlyAdminGuard either")

	// A plain admin, in the same unacknowledged state, must still be gated by
	// compliance — the exemption must be scoped to readonly_admin only, not a
	// general bypass of the guard.
	admin := newAdminRouterWithRoleAndCompliance(service.RoleAdmin)
	w2 := httptest.NewRecorder()
	admin.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil))
	require.Equal(t, http.StatusLocked, w2.Code,
		"a plain admin must still be blocked by ADMIN_COMPLIANCE_ACK_REQUIRED when unacknowledged")
}
