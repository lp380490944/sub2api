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
