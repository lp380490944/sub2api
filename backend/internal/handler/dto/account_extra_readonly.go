package dto

// readonlyAdminExtraAllowlist is the explicit set of Account.Extra keys that
// are safe to return to the readonly_admin role (a customer-facing read-only
// admin account, see task-6b brief under .superpowers/sdd/2026-08-01-readonly-admin-role/).
//
// This is a DEFAULT-DENY allowlist keyed by exact key name — never a denylist,
// never a prefix/substring match. Extra is a free-form map[string]any that
// operators write arbitrary configuration into (and that several import/OAuth
// flows populate with provider-specific fields); a denylist would leak every
// future key by default, which defeats the point of this table. When a key's
// meaning could not be positively established during the task-6b audit, it
// was left off this list — see task-6b-report.md for the full enumeration and
// per-key reasoning.
//
// Only operational/scheduling/quota-style configuration that tells a customer
// how their own traffic is handled belongs here. Anything holding or hinting
// at a credential, cookie, session, token, header value, internal URL,
// hashed secret, PII, or free-form operator note must NOT be added without a
// documented reason in task-6b-report.md.
//
// This is unrelated to redactAccountManagedExtra (which strips
// Ollama-cloud-managed bookkeeping keys for ALL roles, admin included) and to
// RedactCredentials (which strips the separate Credentials field). Real
// admins (service.RoleAdmin) are never filtered through this table — it is
// only applied by the account handler for service.RoleReadonlyAdmin callers.
var readonlyAdminExtraAllowlist = map[string]struct{}{
	// ---- Passthrough / protocol routing toggles ----
	// upstream_passthrough is a closed schema (profile/category_override are
	// validated enums, overrides is a fixed set of named booleans,
	// bedrock_backed_relay is a bool) with no free-text sub-field, so the
	// whole subtree is allowed rather than enumerating sub-keys again here.
	"upstream_passthrough":         {},
	"anthropic_passthrough":        {},
	"anthropic_apikey_auth_scheme": {},
	"openai_passthrough":           {},
	"openai_oauth_passthrough":     {},

	"openai_ws_enabled":                             {},
	"openai_ws_force_http":                          {},
	"openai_ws_allow_store_recovery":                {},
	"openai_apikey_responses_websockets_v2_enabled": {},
	"openai_apikey_responses_websockets_v2_mode":    {},
	"openai_oauth_responses_websockets_v2_enabled":  {},
	"openai_oauth_responses_websockets_v2_mode":     {},
	"responses_websockets_v2_enabled":               {},
	"openai_responses_mode":                         {},
	"openai_responses_supported":                    {},

	"openai_compact_mode":        {},
	"openai_compact_supported":   {},
	"openai_compact_checked_at":  {},
	"openai_compact_last_status": {},

	"codex_cli_only":                              {},
	"codex_cli_only_allow_app_server":             {},
	"codex_cli_only_allowed_clients":              {},
	"codex_image_generation_bridge":               {},
	"codex_image_generation_bridge_enabled":       {},
	"codex_image_generation_explicit_tool_policy": {},

	"openai_long_context_billing_enabled": {},
	"web_search_emulation":                {},
	"mixed_scheduling":                    {},
	"allow_overages":                      {},
	"grok_media_eligible":                 {},
	"grok_client_tool_cache_enabled":      {},
	"synthetic_ui_test":                   {},

	// ---- TLS fingerprint / cache policy ----
	"enable_tls_fingerprint":     {},
	"tls_fingerprint_profile_id": {},
	"cache_ttl_override_enabled": {},
	"cache_ttl_override_target":  {},
	// Only the toggle, never the URL itself (custom_base_url may embed auth
	// in a query string — see the DENY list in task-6b-report.md).
	"custom_base_url_enabled": {},
	"force_1m_context":        {},

	// ---- Session / RPM / concurrency scheduling ----
	"session_id_masking_enabled":   {},
	"max_sessions":                 {},
	"session_idle_timeout_minutes": {},
	"session_window_utilization":   {},
	"base_rpm":                     {},
	"rpm_strategy":                 {},
	"rpm_sticky_buffer":            {},
	"user_msg_queue_mode":          {},
	"user_msg_queue_enabled":       {},
	"window_cost_limit":            {},
	"window_cost_sticky_reserve":   {},
	// Per-model rate-limit reset bookkeeping (model name -> reset info); no
	// free-text sub-fields.
	"model_rate_limits":       {},
	"auto_pause_5h_threshold": {},
	"auto_pause_7d_threshold": {},
	"auto_pause_5h_disabled":  {},
	"auto_pause_7d_disabled":  {},

	// ---- Quota limits / usage / reset schedule (billing-relevant, customer needs this) ----
	"quota_limit":                        {},
	"quota_used":                         {},
	"quota_daily_limit":                  {},
	"quota_daily_used":                   {},
	"quota_daily_start":                  {},
	"quota_daily_reset_mode":             {},
	"quota_daily_reset_hour":             {},
	"quota_daily_reset_at":               {},
	"quota_weekly_limit":                 {},
	"quota_weekly_used":                  {},
	"quota_weekly_start":                 {},
	"quota_weekly_reset_mode":            {},
	"quota_weekly_reset_day":             {},
	"quota_weekly_reset_hour":            {},
	"quota_weekly_reset_at":              {},
	"quota_reset_timezone":               {},
	"quota_notify_daily_enabled":         {},
	"quota_notify_daily_threshold":       {},
	"quota_notify_daily_threshold_type":  {},
	"quota_notify_weekly_enabled":        {},
	"quota_notify_weekly_threshold":      {},
	"quota_notify_weekly_threshold_type": {},
	"quota_notify_total_enabled":         {},
	"quota_notify_total_threshold":       {},
	"quota_notify_total_threshold_type":  {},

	"privacy_mode":       {},
	"subscription_tier":  {},
	"entitlement_status": {},

	"antigravity_credits_overages": {},
	"drive_storage_limit":          {},
	"drive_storage_usage":          {},
	"drive_tier_updated_at":        {},

	"codex_primary_used_percent":           {},
	"codex_primary_reset_after_seconds":    {},
	"codex_primary_window_minutes":         {},
	"codex_secondary_used_percent":         {},
	"codex_secondary_reset_after_seconds":  {},
	"codex_secondary_window_minutes":       {},
	"codex_primary_over_secondary_percent": {},
	"codex_usage_updated_at":               {},
	"codex_5h_used_percent":                {},
	"codex_5h_reset_after_seconds":         {},
	"codex_5h_window_minutes":              {},
	"codex_5h_reset_at":                    {},
	"codex_7d_used_percent":                {},
	"codex_7d_reset_after_seconds":         {},
	"codex_7d_window_minutes":              {},
	"codex_7d_reset_at":                    {},

	"passive_usage_sampled_at":        {},
	"passive_usage_7d_utilization":    {},
	"passive_usage_7d_reset":          {},
	"passive_usage_7d_oi_utilization": {},
	"passive_usage_7d_oi_reset":       {},

	// Pure enable/disable toggle; the probe result blob itself
	// (upstream_billing_probe) is NOT on this list — see task-6b-report.md.
	"upstream_billing_probe_enabled": {},
}

// RedactExtraForReadonlyAdmin filters an Account.Extra map down to the
// readonly_admin-safe allowlist above. Callers apply this only for
// service.RoleReadonlyAdmin; service.RoleAdmin must keep seeing Extra
// untouched (call AccountFromService/AccountFromServiceShallow directly and
// skip this function).
//
// nil in, nil out — mirrors redactAccountManagedExtra's nil handling so
// callers don't need a special case for accounts with no Extra.
func RedactExtraForReadonlyAdmin(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	redacted := make(map[string]any, len(extra))
	for key, value := range extra {
		if _, ok := readonlyAdminExtraAllowlist[key]; ok {
			redacted[key] = value
		}
	}
	return redacted
}
