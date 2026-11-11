//go:build unit

package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestCodexLegacyTUIIdentitySwitch(t *testing.T) {
	const macTUI = "codex-tui/0.0.0 (Mac OS 26.5.1; arm64) iTerm.app/3.6.11 (codex-tui; 0.0.0)"
	legacy := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_identity": "tui"}}
	plain := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	canonical := resolveCodexOutboundIdentity("")
	require.Equal(t, openai.CodexCLIOriginator, canonical.originator)
	require.True(t, strings.HasPrefix(canonical.userAgent, "codex_cli_rs/"))

	t.Run("default follows upstream CLI identity", func(t *testing.T) {
		require.Equal(t, canonical, resolveCodexOutboundIdentity(codexLegacyTUICandidate(plain, "")))
		got := resolveCodexOutboundIdentity(codexLegacyTUICandidate(plain, macTUI))
		require.Equal(t, openai.CodexCLIOriginator, got.originator)
		require.True(t, strings.HasPrefix(got.userAgent, "codex_cli_rs/"))
		require.NotContains(t, got.userAgent, "codex-tui")
	})

	t.Run("legacy account without UA keeps the pre-merge default", func(t *testing.T) {
		got := resolveCodexOutboundIdentity(codexLegacyTUICandidate(legacy, ""))
		require.Equal(t, openai.CodexTUIOriginator, got.originator)
		require.Equal(t, "codex-tui/"+strings.TrimPrefix(canonical.userAgent, "codex_cli_rs/"), got.userAgent)
		require.Equal(t, canonical.version, got.version)
	})

	t.Run("legacy account keeps its explicit TUI UA shape", func(t *testing.T) {
		got := resolveCodexOutboundIdentity(codexLegacyTUICandidate(legacy, macTUI))
		require.Equal(t, openai.CodexTUIOriginator, got.originator)
		require.True(t, strings.HasPrefix(got.userAgent, "codex-tui/"))
		require.Contains(t, got.userAgent, "(Mac OS 26.5.1; arm64) iTerm.app/3.6.11 (codex-tui; ")
	})

	t.Run("marker never reaches the wire", func(t *testing.T) {
		for _, account := range []*Account{legacy, plain, nil} {
			h := http.Header{}
			h.Set("originator", "anything")
			enforceCodexIdentityHeadersWithUA(h, codexLegacyTUICandidate(account, macTUI))
			for _, k := range []string{"user-agent", "originator", "version"} {
				require.NotContains(t, h.Get(k), "\x00", k)
				require.NotContains(t, h.Get(k), "fork-legacy", k)
			}
		}
	})

	t.Run("flag parsing", func(t *testing.T) {
		require.True(t, (&Account{Extra: map[string]any{"codex_identity": " TUI "}}).KeepsLegacyCodexTUIIdentity())
		require.False(t, (&Account{Extra: map[string]any{"codex_identity": "cli"}}).KeepsLegacyCodexTUIIdentity())
		require.False(t, (*Account)(nil).KeepsLegacyCodexTUIIdentity())
	})
}
