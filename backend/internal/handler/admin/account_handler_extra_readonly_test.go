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

// accountWithCustomBaseURLFixture builds an Anthropic OAuth account (the only
// account shape for which dto.Account.CustomBaseURL is populated — see
// AccountFromServiceShallow's IsAnthropicOAuthOrSetupToken() gate) with a
// custom relay base URL configured. Used by the CRITICAL-fix regression
// tests: CustomBaseURL is a typed top-level field, separate from Extra, that
// mirrors the denied Extra["custom_base_url"] key under a different JSON
// name.
func accountWithCustomBaseURLFixture(id int64) *service.Account {
	return &service.Account{
		ID:       id,
		Name:     "acct",
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Extra: map[string]any{
			"custom_base_url_enabled": true,
			"custom_base_url":         "https://relay.internal/?key=sk-embedded-secret",
			"enable_tls_fingerprint":  true,
		},
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

	// Body-level (not just decoded-struct-level): the raw serialized response
	// must not contain the denied key/value as text anywhere in the payload.
	raw := w.Body.String()
	require.NotContainsf(t, raw, "secret_header", "denied key must not appear anywhere in the raw response body: %s", raw)
	require.NotContainsf(t, raw, "sk-should-not-leak", "denied value must not appear anywhere in the raw response body: %s", raw)
	require.NotContainsf(t, raw, "registration_fingerprint", "denied key must not appear anywhere in the raw response body: %s", raw)
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

	// Body-level (not just decoded-struct-level): the raw serialized response
	// must not contain the denied key/value as text anywhere in the payload.
	raw := w.Body.String()
	require.NotContainsf(t, raw, "secret_header", "denied key must not appear anywhere in the raw response body: %s", raw)
	require.NotContainsf(t, raw, "sk-should-not-leak", "denied value must not appear anywhere in the raw response body: %s", raw)
	require.NotContainsf(t, raw, "registration_fingerprint", "denied key must not appear anywhere in the raw response body: %s", raw)
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

// TestAccountHandlerGetByID_ReadonlyAdminHidesCustomBaseURL is the
// CRITICAL-fix regression test: CustomBaseURL is a typed top-level field
// (json:"custom_base_url"), NOT part of the Extra map, populated from the
// same Extra["custom_base_url"] value this task denies. A redaction that
// only touched the Extra map would still leak the URL through this parallel
// path. Asserts at both the decoded-struct level and the raw response-body
// level (the exact top-level JSON key, not the enabled-toggle field whose
// name is a superstring of it, must be absent).
func TestAccountHandlerGetByID_ReadonlyAdminHidesCustomBaseURL(t *testing.T) {
	stub := newStubAdminService()
	stub.getAccountResult = accountWithCustomBaseURLFixture(9)
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts/9", service.RoleReadonlyAdmin)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	h.GetByID(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeAccountEnvelope(t, w.Body.Bytes())

	require.Nilf(t, body.CustomBaseURL, "CustomBaseURL must be nilled for readonly_admin, got %#v", body.CustomBaseURL)
	require.NotNilf(t, body.CustomBaseURLEnabled, "CustomBaseURLEnabled toggle must survive")
	require.Truef(t, body.CustomBaseURLEnabled != nil && *body.CustomBaseURLEnabled, "CustomBaseURLEnabled must remain true")
	require.NotContains(t, body.Extra, "custom_base_url")
	require.Contains(t, body.Extra, "custom_base_url_enabled")

	raw := w.Body.String()
	require.NotContainsf(t, raw, `"custom_base_url":`, "exact top-level key custom_base_url must be absent from the raw body: %s", raw)
	require.NotContainsf(t, raw, "sk-embedded-secret", "URL value must not appear anywhere in the raw response body: %s", raw)
	require.Containsf(t, raw, `"custom_base_url_enabled":true`, "enabled toggle must survive in the raw body: %s", raw)
}

// TestAccountHandlerGetByID_AdminSeesCustomBaseURL is the regression guard for
// the fix above: role admin must keep seeing CustomBaseURL untouched.
func TestAccountHandlerGetByID_AdminSeesCustomBaseURL(t *testing.T) {
	stub := newStubAdminService()
	stub.getAccountResult = accountWithCustomBaseURLFixture(9)
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts/9", service.RoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	h.GetByID(c)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeAccountEnvelope(t, w.Body.Bytes())

	require.NotNilf(t, body.CustomBaseURL, "admin must still see CustomBaseURL")
	require.Equal(t, "https://relay.internal/?key=sk-embedded-secret", *body.CustomBaseURL)
	require.Contains(t, body.Extra, "custom_base_url")

	raw := w.Body.String()
	require.Containsf(t, raw, `"custom_base_url":"https://relay.internal`, "admin body must still contain the URL: %s", raw)
}

// TestAccountHandlerList_ReadonlyAdminHidesCustomBaseURL is the List-endpoint
// sibling of the GetByID CustomBaseURL regression test.
func TestAccountHandlerList_ReadonlyAdminHidesCustomBaseURL(t *testing.T) {
	stub := newStubAdminService()
	stub.accounts = []service.Account{*accountWithCustomBaseURLFixture(10)}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", service.RoleReadonlyAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeAccountListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)

	require.Nilf(t, items[0].CustomBaseURL, "CustomBaseURL must be nilled for readonly_admin, got %#v", items[0].CustomBaseURL)
	require.NotNilf(t, items[0].CustomBaseURLEnabled, "CustomBaseURLEnabled toggle must survive")

	raw := w.Body.String()
	require.NotContainsf(t, raw, `"custom_base_url":`, "exact top-level key custom_base_url must be absent from the raw body: %s", raw)
	require.NotContainsf(t, raw, "sk-embedded-secret", "URL value must not appear anywhere in the raw response body: %s", raw)
}

// TestAccountHandlerList_AdminSeesCustomBaseURL is the List-endpoint sibling
// of the admin regression guard above.
func TestAccountHandlerList_AdminSeesCustomBaseURL(t *testing.T) {
	stub := newStubAdminService()
	stub.accounts = []service.Account{*accountWithCustomBaseURLFixture(10)}
	h := newAccountHandlerForExtraTest(stub)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20", service.RoleAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeAccountListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)
	require.NotNilf(t, items[0].CustomBaseURL, "admin must still see CustomBaseURL")
}

type accountEnvelopeBody struct {
	Extra                map[string]any `json:"extra"`
	CustomBaseURL        *string        `json:"custom_base_url"`
	CustomBaseURLEnabled *bool          `json:"custom_base_url_enabled"`
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
