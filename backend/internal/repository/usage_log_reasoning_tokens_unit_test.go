//go:build unit

package repository

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// usageLogColumnScanner fills every Scan destination with its zero value,
// except for columns listed in overrides (keyed by usageLogSelectColumns name).
type usageLogColumnScanner struct {
	columns   []string
	overrides map[string]any
}

func (s usageLogColumnScanner) Scan(dest ...any) error {
	if len(dest) != len(s.columns) {
		return fmt.Errorf("scan arg count mismatch: got %d want %d", len(dest), len(s.columns))
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Pointer {
			return fmt.Errorf("dest[%d] is not pointer", i)
		}
		if v, ok := s.overrides[s.columns[i]]; ok {
			dv.Elem().Set(reflect.ValueOf(v))
			continue
		}
		dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
	}
	return nil
}

func usageLogSelectColumnList() []string {
	parts := strings.Split(usageLogSelectColumns, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func reasoningTokensArgIndex(t *testing.T) int {
	t.Helper()
	// usageLogInsertArgTypes omits id, so its index is select index - 1.
	for i, col := range usageLogSelectColumnList() {
		if col == "reasoning_tokens" {
			return i - 1
		}
	}
	t.Fatal("reasoning_tokens missing from usageLogSelectColumns")
	return -1
}

func TestPrepareUsageLogInsert_ReasoningTokensArgWiring(t *testing.T) {
	idx := reasoningTokensArgIndex(t)
	require.Equal(t, "integer", usageLogInsertArgTypes[idx], "reasoning_tokens arg type must be integer")
	require.Equal(t, "text", usageLogInsertArgTypes[idx-1], "reasoning_tokens must follow requested_reasoning_effort")

	reasoning := 1234
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:          1,
		APIKeyID:        2,
		AccountID:       3,
		RequestID:       "req-reasoning-tokens",
		Model:           "gpt-5.4",
		ReasoningTokens: &reasoning,
		CreatedAt:       time.Now().UTC(),
	})
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
	arg, ok := prepared.args[idx].(sql.NullInt64)
	require.True(t, ok)
	require.True(t, arg.Valid)
	require.Equal(t, int64(reasoning), arg.Int64)

	preparedNil := prepareUsageLogInsert(&service.UsageLog{
		UserID:    1,
		APIKeyID:  2,
		AccountID: 3,
		RequestID: "req-reasoning-tokens-nil",
		Model:     "gpt-5.4",
		CreatedAt: time.Now().UTC(),
	})
	nilArg, ok := preparedNil.args[idx].(sql.NullInt64)
	require.True(t, ok)
	require.False(t, nilArg.Valid, "unknown reasoning tokens must persist as NULL")
}

func TestUsageLogInsertQueries_IncludeReasoningTokens(t *testing.T) {
	require.Contains(t, usageLogSelectColumns, "reasoning_tokens")

	reasoning := 7
	log := &service.UsageLog{
		UserID:          1,
		APIKeyID:        2,
		AccountID:       3,
		RequestID:       "req-reasoning-tokens-query",
		Model:           "gpt-5.4",
		ReasoningTokens: &reasoning,
		CreatedAt:       time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)
	key := usageLogBatchKey(log.RequestID, log.APIKeyID)

	batchQuery, batchArgs := buildUsageLogBatchInsertQuery([]string{key},
		map[string]usageLogInsertPrepared{key: prepared})
	require.GreaterOrEqual(t, strings.Count(batchQuery, "reasoning_tokens"), 3)
	require.Len(t, batchArgs, len(prepared.args)+1)

	bestEffortQuery, bestEffortArgs := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(bestEffortQuery, "reasoning_tokens"), 3)
	require.Len(t, bestEffortArgs, len(prepared.args))
}

func TestScanUsageLog_ReasoningTokens(t *testing.T) {
	columns := usageLogSelectColumnList()
	now := time.Now().UTC()

	log, err := scanUsageLog(usageLogColumnScanner{columns: columns, overrides: map[string]any{
		"model":            "gpt-5.4",
		"reasoning_tokens": sql.NullInt64{Valid: true, Int64: 4321},
		"created_at":       now,
	}})
	require.NoError(t, err)
	require.NotNil(t, log.ReasoningTokens)
	require.Equal(t, 4321, *log.ReasoningTokens)

	logNull, err := scanUsageLog(usageLogColumnScanner{columns: columns, overrides: map[string]any{
		"model":      "gpt-5.4",
		"created_at": now,
	}})
	require.NoError(t, err)
	require.Nil(t, logNull.ReasoningTokens)
}
