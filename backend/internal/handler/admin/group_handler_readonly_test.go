package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// groupWithEmbeddedAccountFixture builds a service.Group whose AccountGroups
// slice carries a fully populated Account (Extra with a denied key, and a
// CustomBaseURL-shaped setup). Production's group repository never populates
// Group.AccountGroups for the read endpoints under test, but this fixture
// constructs the shape directly so the test is meaningful regardless — see
// dto.RedactAccountGroupsForReadonlyAdmin's doc comment for why that matters.
func groupWithEmbeddedAccountFixture() service.Group {
	url := "https://relay.internal/?key=sk-embedded-secret"
	return service.Group{
		ID:     1,
		Name:   "group-with-accounts",
		Status: service.StatusActive,
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 9,
				GroupID:   1,
				Account: &service.Account{
					ID:       9,
					Name:     "acct",
					Platform: service.PlatformAnthropic,
					Type:     service.AccountTypeOAuth,
					Status:   service.StatusActive,
					Extra: map[string]any{
						"custom_base_url_enabled": true,
						"custom_base_url":         url,
						"enable_tls_fingerprint":  true,
						"secret_header":           "Bearer sk-should-not-leak",
					},
				},
			},
		},
	}
}

type groupListEnvelopeItem struct {
	ID            int64 `json:"id"`
	AccountGroups []struct {
		AccountID int64 `json:"account_id"`
		Account   *struct {
			Extra         map[string]any `json:"extra"`
			CustomBaseURL *string        `json:"custom_base_url"`
		} `json:"account"`
	} `json:"account_groups"`
}

func decodeGroupListEnvelope(t *testing.T, raw []byte) []groupListEnvelopeItem {
	t.Helper()
	var payload struct {
		Data struct {
			Items []groupListEnvelopeItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	return payload.Data.Items
}

// TestGroupHandlerList_ReadonlyAdminRedactsEmbeddedAccount is the regression
// test for the latent leak in finding 3: AccountGroupFromService always
// embeds a full, otherwise-unredacted dto.Account on AccountGroup.Account.
// GET /groups is on the readonly_admin allowlist, so if that field is ever
// populated (it is not, today), it must come back redacted.
func TestGroupHandlerList_ReadonlyAdminRedactsEmbeddedAccount(t *testing.T) {
	stub := newStubAdminService()
	stub.groups = []service.Group{groupWithEmbeddedAccountFixture()}
	h := NewGroupHandler(stub, nil, nil)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/groups?page=1&page_size=20", service.RoleReadonlyAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeGroupListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)
	require.Len(t, items[0].AccountGroups, 1)
	acc := items[0].AccountGroups[0].Account
	require.NotNilf(t, acc, "embedded account must still be present, just redacted: %#v", items[0].AccountGroups[0])

	require.Nilf(t, acc.CustomBaseURL, "embedded Account.CustomBaseURL must be nilled for readonly_admin, got %#v", acc.CustomBaseURL)
	require.NotContainsf(t, acc.Extra, "custom_base_url", "embedded Account Extra[custom_base_url] must be dropped: %#v", acc.Extra)
	require.NotContainsf(t, acc.Extra, "secret_header", "embedded Account Extra[secret_header] must be dropped: %#v", acc.Extra)
	require.Containsf(t, acc.Extra, "enable_tls_fingerprint", "allowlisted key must survive: %#v", acc.Extra)

	raw := w.Body.String()
	require.NotContainsf(t, raw, "sk-should-not-leak", "denied value must not appear anywhere in the raw response body: %s", raw)
	require.NotContainsf(t, raw, "sk-embedded-secret", "denied URL must not appear anywhere in the raw response body: %s", raw)
}

// TestGroupHandlerList_AdminSeesEmbeddedAccountUnredacted is the regression
// guard: role admin must keep seeing the embedded account exactly as built,
// proving the redaction added for finding 3 is scoped to readonly_admin only.
func TestGroupHandlerList_AdminSeesEmbeddedAccountUnredacted(t *testing.T) {
	stub := newStubAdminService()
	stub.groups = []service.Group{groupWithEmbeddedAccountFixture()}
	h := NewGroupHandler(stub, nil, nil)

	c, w := accountExtraTestContext(http.MethodGet, "/api/v1/admin/groups?page=1&page_size=20", service.RoleAdmin)
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	items := decodeGroupListEnvelope(t, w.Body.Bytes())
	require.Len(t, items, 1)
	require.Len(t, items[0].AccountGroups, 1)
	acc := items[0].AccountGroups[0].Account
	require.NotNil(t, acc)

	require.NotNilf(t, acc.CustomBaseURL, "admin must still see the embedded account's CustomBaseURL")
	require.Contains(t, acc.Extra, "custom_base_url")
	require.Contains(t, acc.Extra, "secret_header")
}
