package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// fork: per-account opt-out of upstream's TUI -> CLI identity normalization.
//
// Upstream (c591412c4) switched the gateway default Codex identity from
// codex-tui to codex_cli_rs and rewrites every codex-tui identity to CLI. That
// flips the originator / User-Agent of all OAuth accounts at once. Accounts whose
// extra carries codex_identity="tui" keep the exact pre-merge wire identity so the
// change can be rolled out account by account (docs/fork/CODEX_IDENTITY_EMULATION.md §2).
const (
	codexIdentityExtraKey     = "codex_identity"
	codexIdentityLegacyTUI    = "tui"
	codexLegacyTUICandidateNS = "\x00fork-legacy-tui\x00"
)

// KeepsLegacyCodexTUIIdentity reports whether the account is pinned to the
// pre-merge codex-tui identity.
func (a *Account) KeepsLegacyCodexTUIIdentity() bool {
	if a == nil || a.Extra == nil {
		return false
	}
	raw, _ := a.Extra[codexIdentityExtraKey].(string)
	return strings.EqualFold(strings.TrimSpace(raw), codexIdentityLegacyTUI)
}

// codexLegacyTUICandidate tags a candidate User-Agent for resolveCodexOutboundIdentity
// when the account is pinned to the legacy identity. The tag never reaches the wire:
// resolveCodexOutboundIdentity strips it before any header is built.
func codexLegacyTUICandidate(account *Account, candidateUA string) string {
	if !account.KeepsLegacyCodexTUIIdentity() {
		return candidateUA
	}
	return codexLegacyTUICandidateNS + candidateUA
}

func splitCodexLegacyTUICandidate(candidateUA string) (string, bool) {
	return strings.CutPrefix(candidateUA, codexLegacyTUICandidateNS)
}

// codexLegacyTUIIdentity restores the pre-merge behaviour: an explicit account
// User-Agent is used as paired; the gateway default (now CLI) becomes codex-tui
// with the same version / OS / terminal segments.
func codexLegacyTUIIdentity(originator, pairedUA string, fromCanonical bool) (string, string) {
	if !fromCanonical || !strings.EqualFold(originator, openai.CodexCLIOriginator) {
		return originator, pairedUA
	}
	slash := strings.IndexByte(pairedUA, '/')
	if slash <= 0 {
		return originator, pairedUA
	}
	return openai.CodexTUIOriginator, openai.CodexTUIOriginator + pairedUA[slash:]
}
