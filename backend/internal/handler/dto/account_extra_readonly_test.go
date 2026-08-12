package dto

import "testing"

// TestRedactExtraForReadonlyAdmin_AllowsKnownSafeKeys verifies that keys on the
// readonly_admin Extra allowlist survive filtering. Fails if the allowlist
// wrongly omits a key that operators and the frontend actually rely on.
func TestRedactExtraForReadonlyAdmin_AllowsKnownSafeKeys(t *testing.T) {
	extra := map[string]any{
		"upstream_passthrough": map[string]any{
			"profile":              "transparent",
			"category_override":    "relay",
			"bedrock_backed_relay": true,
			"overrides": map[string]any{
				"forward_client_headers": true,
			},
		},
		"enable_tls_fingerprint": true,
		"force_1m_context":       true,
		"quota_daily_limit":      100.0,
		"quota_daily_used":       12.5,
	}

	got := RedactExtraForReadonlyAdmin(extra)

	if _, ok := got["upstream_passthrough"]; !ok {
		t.Fatalf("expected upstream_passthrough to survive redaction, got %#v", got)
	}
	if _, ok := got["enable_tls_fingerprint"]; !ok {
		t.Fatalf("expected enable_tls_fingerprint to survive redaction, got %#v", got)
	}
	if _, ok := got["force_1m_context"]; !ok {
		t.Fatalf("expected force_1m_context to survive redaction, got %#v", got)
	}
	if _, ok := got["quota_daily_limit"]; !ok {
		t.Fatalf("expected quota_daily_limit to survive redaction, got %#v", got)
	}
	if _, ok := got["quota_daily_used"]; !ok {
		t.Fatalf("expected quota_daily_used to survive redaction, got %#v", got)
	}

	up, ok := got["upstream_passthrough"].(map[string]any)
	if !ok {
		t.Fatalf("expected upstream_passthrough to be a map, got %#v", got["upstream_passthrough"])
	}
	if up["profile"] != "transparent" || up["category_override"] != "relay" || up["bedrock_backed_relay"] != true {
		t.Fatalf("expected all four known sub-keys to survive, got %#v", up)
	}
	if _, ok := up["overrides"]; !ok {
		t.Fatalf("expected overrides sub-key to survive, got %#v", up)
	}
}

// TestRedactExtraForReadonlyAdmin_UpstreamPassthroughDropsUnknownSubKeys
// proves upstream_passthrough is filtered sub-key-by-sub-key (the schema is
// enumerated, not allowed wholesale): an unexpected sub-key injected into the
// stored object (e.g. by a malformed direct write) must not survive, even
// though the top-level key itself is allowlisted.
func TestRedactExtraForReadonlyAdmin_UpstreamPassthroughDropsUnknownSubKeys(t *testing.T) {
	extra := map[string]any{
		"upstream_passthrough": map[string]any{
			"profile":       "transparent",
			"smuggled_note": "sk-should-not-leak",
		},
	}

	got := RedactExtraForReadonlyAdmin(extra)

	up, ok := got["upstream_passthrough"].(map[string]any)
	if !ok {
		t.Fatalf("expected upstream_passthrough to survive (has a known sub-key), got %#v", got)
	}
	if _, ok := up["profile"]; !ok {
		t.Fatalf("expected known sub-key profile to survive, got %#v", up)
	}
	if _, ok := up["smuggled_note"]; ok {
		t.Fatalf("expected unknown sub-key smuggled_note to be dropped, got %#v", up)
	}
}

// TestRedactExtraForReadonlyAdmin_DropsDeadCredentialsOnlyKeys covers the two
// review-flagged dead rows: subscription_tier and entitlement_status are
// Credentials keys (grok_oauth_service.go), never Extra keys, and must not be
// on the Extra allowlist (a row there would be inert but misleading).
// codex_cli_only_allowed_clients is a deprecated, backend-unread,
// operator-typed field and must be denied per "when in doubt, deny".
func TestRedactExtraForReadonlyAdmin_DropsDeadCredentialsOnlyKeys(t *testing.T) {
	extra := map[string]any{
		"subscription_tier":              "premium",
		"entitlement_status":             "active",
		"codex_cli_only_allowed_clients": []any{"codex_cli"},
	}

	got := RedactExtraForReadonlyAdmin(extra)

	for _, denied := range []string{"subscription_tier", "entitlement_status", "codex_cli_only_allowed_clients"} {
		if _, ok := got[denied]; ok {
			t.Fatalf("expected %q to be dropped, got %#v", denied, got)
		}
	}
}

// TestRedactExtraForReadonlyAdmin_DropsUnknownAndSensitiveKeys is the
// default-deny property test: a made-up key that appears nowhere in the
// codebase (secret_header) must be dropped, alongside a key with an
// established-sensitive meaning (registration_fingerprint) and an
// operator-note-style key (operator_note).
func TestRedactExtraForReadonlyAdmin_DropsUnknownAndSensitiveKeys(t *testing.T) {
	extra := map[string]any{
		"enable_tls_fingerprint":   true, // safe control, should remain
		"secret_header":            "Bearer sk-should-not-leak",
		"operator_note":            "internal note about this account",
		"registration_fingerprint": map[string]any{"os": "MacOS"},
		"custom_base_url":          "https://relay.internal/?key=sk-embedded-secret",
	}

	got := RedactExtraForReadonlyAdmin(extra)

	if _, ok := got["enable_tls_fingerprint"]; !ok {
		t.Fatalf("expected enable_tls_fingerprint to survive redaction, got %#v", got)
	}
	for _, denied := range []string{"secret_header", "operator_note", "registration_fingerprint", "custom_base_url"} {
		if _, ok := got[denied]; ok {
			t.Fatalf("expected %q to be dropped by default-deny redaction, got %#v", denied, got)
		}
	}
}

// TestRedactExtraForReadonlyAdmin_NilExtraReturnsNil guards against a panic on
// accounts with no Extra map (the common case).
func TestRedactExtraForReadonlyAdmin_NilExtraReturnsNil(t *testing.T) {
	if got := RedactExtraForReadonlyAdmin(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %#v", got)
	}
}

// TestRedactAccountForReadonlyAdmin_NilsCustomBaseURLButKeepsEnabledToggle is
// the CRITICAL-fix regression test: CustomBaseURL is a typed top-level field
// (not part of Extra) populated from the same denied Extra["custom_base_url"]
// value via GetCustomBaseURL(). RedactExtraForReadonlyAdmin alone cannot see
// or clear it — only RedactAccountForReadonlyAdmin, operating on the whole
// *Account, can. CustomBaseURLEnabled (the toggle) must survive untouched.
func TestRedactAccountForReadonlyAdmin_NilsCustomBaseURLButKeepsEnabledToggle(t *testing.T) {
	url := "https://relay.internal/?key=sk-embedded-secret"
	enabled := true
	out := &Account{
		CustomBaseURL:        &url,
		CustomBaseURLEnabled: &enabled,
		Extra: map[string]any{
			"custom_base_url_enabled": true,
			"custom_base_url":         url,
			"enable_tls_fingerprint":  true,
		},
	}

	RedactAccountForReadonlyAdmin(out)

	if out.CustomBaseURL != nil {
		t.Fatalf("expected CustomBaseURL to be nilled out, got %q", *out.CustomBaseURL)
	}
	if out.CustomBaseURLEnabled == nil || !*out.CustomBaseURLEnabled {
		t.Fatalf("expected CustomBaseURLEnabled to survive untouched, got %#v", out.CustomBaseURLEnabled)
	}
	if _, ok := out.Extra["custom_base_url"]; ok {
		t.Fatalf("expected Extra[custom_base_url] to be dropped, got %#v", out.Extra)
	}
	if _, ok := out.Extra["custom_base_url_enabled"]; !ok {
		t.Fatalf("expected Extra[custom_base_url_enabled] to survive, got %#v", out.Extra)
	}
	if _, ok := out.Extra["enable_tls_fingerprint"]; !ok {
		t.Fatalf("expected Extra[enable_tls_fingerprint] to survive, got %#v", out.Extra)
	}
}

// TestRedactAccountForReadonlyAdmin_NilAccountNoop guards against a panic
// when out is nil (defensive; callers already nil-check before calling, but
// this mirrors the nil-safety of every other helper in this file).
func TestRedactAccountForReadonlyAdmin_NilAccountNoop(t *testing.T) {
	RedactAccountForReadonlyAdmin(nil) // must not panic
}

// TestRedactAccountGroupsForReadonlyAdmin_RedactsEmbeddedAccount guards the
// latent leak path: AccountGroupFromService always embeds a full, otherwise
// UNREDACTED Account DTO on AccountGroup.Account (reachable via
// AdminGroup.AccountGroups on GET /groups, /groups/all, /groups/:id). It is
// unreachable in production today only because the group repository never
// populates Group.AccountGroups for those endpoints — an incidental gap, not
// a guarantee. This test is meaningful precisely because it does not depend
// on that gap: it constructs the DTO shape directly, so a future eager-load
// that starts populating the field is covered from day one.
func TestRedactAccountGroupsForReadonlyAdmin_RedactsEmbeddedAccount(t *testing.T) {
	url := "https://relay.internal/?key=sk-embedded-secret"
	groups := []AccountGroup{
		{
			AccountID: 1,
			GroupID:   1,
			Account: &Account{
				ID:            1,
				CustomBaseURL: &url,
				Extra: map[string]any{
					"custom_base_url":        url,
					"enable_tls_fingerprint": true,
					"secret_header":          "Bearer sk-should-not-leak",
				},
			},
		},
		// A nil embedded Account (the production-typical shape today) must not
		// panic.
		{AccountID: 2, GroupID: 1, Account: nil},
	}

	RedactAccountGroupsForReadonlyAdmin(groups)

	acc := groups[0].Account
	if acc.CustomBaseURL != nil {
		t.Fatalf("expected embedded Account.CustomBaseURL to be nilled, got %q", *acc.CustomBaseURL)
	}
	if _, ok := acc.Extra["custom_base_url"]; ok {
		t.Fatalf("expected embedded Account Extra[custom_base_url] to be dropped, got %#v", acc.Extra)
	}
	if _, ok := acc.Extra["secret_header"]; ok {
		t.Fatalf("expected embedded Account Extra[secret_header] to be dropped, got %#v", acc.Extra)
	}
	if _, ok := acc.Extra["enable_tls_fingerprint"]; !ok {
		t.Fatalf("expected allowlisted Extra[enable_tls_fingerprint] to survive, got %#v", acc.Extra)
	}
}

// TestRedactAccountGroupsForReadonlyAdmin_NilSliceNoop guards nil-safety.
func TestRedactAccountGroupsForReadonlyAdmin_NilSliceNoop(t *testing.T) {
	RedactAccountGroupsForReadonlyAdmin(nil) // must not panic
}
