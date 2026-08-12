package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func AdminComplianceGuard(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingService == nil || isAdminComplianceBypassPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// readonly_admin 是只读客户角色，无法执行任何变更操作；部署与运营合规承诺
		// 约束的是运营者（部署这套系统的人），不是被邀请进面板的只读客户，让其代为
		// 签署没有意义，也没有退路（/compliance* 端点本身被只读白名单拒绝）。
		// 因此在这里豁免该角色，其余角色（含 admin）不受影响，仍需正常确认。
		if role, ok := GetUserRoleFromContext(c); ok && role == service.RoleReadonlyAdmin {
			c.Next()
			return
		}

		subject, ok := GetAuthSubjectFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}

		acknowledged, err := settingService.IsAdminComplianceAcknowledged(c.Request.Context(), subject.UserID)
		if err != nil {
			AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}
		if acknowledged {
			c.Next()
			return
		}

		c.JSON(http.StatusLocked, gin.H{
			"code":    "ADMIN_COMPLIANCE_ACK_REQUIRED",
			"message": "administrator compliance acknowledgement is required",
			"metadata": gin.H{
				"version":          service.AdminComplianceVersion,
				"document_path_zh": service.AdminComplianceDocumentPathZH,
				"document_path_en": service.AdminComplianceDocumentPathEN,
				"document_url_zh":  service.AdminComplianceDocumentURLZH,
				"document_url_en":  service.AdminComplianceDocumentURLEN,
			},
		})
		c.Abort()
	}
}

func isAdminComplianceBypassPath(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/api/v1/admin/compliance" || strings.HasPrefix(path, "/api/v1/admin/compliance/")
}
