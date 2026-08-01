package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// accountExtraTestFixture returns an Extra map holding one allowlisted key
// (enable_tls_fingerprint), one made-up key absent from the entire codebase
// (secret_header — the default-deny property), and one established-sensitive
// key (registration_fingerprint).
func accountExtraTestFixture() map[string]any {
	return map[string]any{
		"enable_tls_fingerprint":   true,
		"secret_header":            "Bearer sk-should-not-leak",
		"registration_fingerprint": map[string]any{"os": "MacOS"},
	}
}

func newAccountHandlerForExtraTest(stub *stubAdminService) *AccountHandler {
	return NewAccountHandler(stub, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func accountExtraTestContext(method, target string, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})
	c.Set(string(middleware2.ContextKeyUserRole), role)
	return c, w
}

// TestAccountHandlerGetByID_ReadonlyAdminExtraRedaction proves the redaction is
// wired into GetByID: as readonly_admin the safe key survives and both the
// unknown made-up key and the established-sensitive key are stripped. This
// test fails if the readonly redaction step is removed from GetByID.
func TestAccountHandlerGetByID_ReadonlyAdminExtraRedaction(t *testing.T) {
	stub := newStubAdminService()
	stub.getAccountResult = &service.Account{
		ID:       7,
		Name:     "acct",
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Extra:    accountExtraTestFixture(),
	}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts/7", service.RoleReadonlyAdmin)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	h.GetByID(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeAccountEnvelope(t, w.Body.Bytes())

	require.Containsf(t, body.Extra, "enable_tls_fingerprint", "allowlisted key must survive: %#v", body.Extra)
	require.NotContainsf(t, body.Extra, "secret_header", "unlisted key must be dropped by default-deny: %#v", body.Extra)
	require.NotContainsf(t, body.Extra, "registration_fingerprint", "sensitive key must be dropped: %#v", body.Extra)
}

// TestAccountHandlerGetByID_AdminSeesExtraUnredacted is the regression guard:
// role admin must see Extra exactly as stored, including keys absent from the
// readonly allowlist. If the redaction leaked into the admin path, this test
// fails.
func TestAccountHandlerGetByID_AdminSeesExtraUnredacted(t *testing.T) {
	stub := newStubAdminService()
	stub.getAccountResult = &service.Account{
		ID:       7,
		Name:     "acct",
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Extra:    accountExtraTestFixture(),
	}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts/7", service.RoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	h.GetByID(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeAccountEnvelope(t, w.Body.Bytes())

	require.Contains(t, body.Extra, "enable_tls_fingerprint")
	require.Contains(t, body.Extra, "secret_header")
	require.Contains(t, body.Extra, "registration_fingerprint")
}

// TestAccountHandlerList_ReadonlyAdminExtraRedaction mirrors the GetByID
// coverage for the paginated list endpoint.
func TestAccountHandlerList_ReadonlyAdminExtraRedaction(t *testing.T) {
	stub := newStubAdminService()
	stub.accounts = []service.Account{
		{
			ID:       8,
			Name:     "acct",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Extra:    accountExtraTestFixture(),
		},
	}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", service.RoleReadonlyAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeAccountListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)

	require.Containsf(t, items[0].Extra, "enable_tls_fingerprint", "allowlisted key must survive: %#v", items[0].Extra)
	require.NotContainsf(t, items[0].Extra, "secret_header", "unlisted key must be dropped by default-deny: %#v", items[0].Extra)
	require.NotContainsf(t, items[0].Extra, "registration_fingerprint", "sensitive key must be dropped: %#v", items[0].Extra)
}

// TestAccountHandlerList_AdminSeesExtraUnredacted is the List-endpoint sibling
// of the admin regression guard above.
func TestAccountHandlerList_AdminSeesExtraUnredacted(t *testing.T) {
	stub := newStubAdminService()
	stub.accounts = []service.Account{
		{
			ID:       8,
			Name:     "acct",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Extra:    accountExtraTestFixture(),
		},
	}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", service.RoleAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeAccountListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)

	require.Contains(t, items[0].Extra, "enable_tls_fingerprint")
	require.Contains(t, items[0].Extra, "secret_header")
	require.Contains(t, items[0].Extra, "registration_fingerprint")
}

type accountEnvelopeBody struct {
	Extra map[string]any `json:"extra"`
}

func decodeAccountEnvelope(t *testing.T, raw []byte) accountEnvelopeBody {
	t.Helper()
	var payload struct {
		Data accountEnvelopeBody `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload.Data
}

func decodeAccountListEnvelope(t *testing.T, raw []byte) []accountEnvelopeBody {
	t.Helper()
	var payload struct {
		Data struct {
			Items []accountEnvelopeBody `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload.Data.Items
}
