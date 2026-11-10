package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIUsageFromGJSON_ReasoningTokens(t *testing.T) {
	tests := []struct {
		name          string
		usage         string
		wantReasoning int
		wantOutput    int
	}{
		{
			name:          "responses output_tokens_details",
			usage:         `{"input_tokens":100,"output_tokens":60,"total_tokens":160,"output_tokens_details":{"reasoning_tokens":40}}`,
			wantReasoning: 40,
			wantOutput:    60,
		},
		{
			name:          "chat completions completion_tokens_details",
			usage:         `{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18,"completion_tokens_details":{"reasoning_tokens":5}}`,
			wantReasoning: 5,
			wantOutput:    8,
		},
		{
			name:          "missing details",
			usage:         `{"input_tokens":3,"output_tokens":4}`,
			wantReasoning: 0,
			wantOutput:    4,
		},
		{
			name:          "xai independent reasoning keeps billing semantics",
			usage:         `{"prompt_tokens":10,"completion_tokens":8,"total_tokens":38,"completion_tokens_details":{"reasoning_tokens":20}}`,
			wantReasoning: 20,
			wantOutput:    28,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, ok := openAIUsageFromGJSON(gjson.Parse(tt.usage))
			require.True(t, ok)
			require.Equal(t, tt.wantReasoning, usage.ReasoningTokens)
			require.Equal(t, tt.wantOutput, usage.OutputTokens)
		})
	}
}

func TestExtractOpenAIUsageFromJSONBytes_ResponsesReasoningTokens(t *testing.T) {
	body := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":50,"output_tokens":30,"total_tokens":80,"output_tokens_details":{"reasoning_tokens":22}}}}`)
	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	require.True(t, ok)
	require.Equal(t, 22, usage.ReasoningTokens)
	require.Equal(t, 30, usage.OutputTokens)
}

func TestMergeOpenAIUsageNonZero_ReasoningTokens(t *testing.T) {
	dst := OpenAIUsage{InputTokens: 1, ReasoningTokens: 7}
	mergeOpenAIUsageNonZero(&dst, OpenAIUsage{OutputTokens: 3})
	require.Equal(t, 7, dst.ReasoningTokens, "zero source must not clear reasoning tokens")
	mergeOpenAIUsageNonZero(&dst, OpenAIUsage{ReasoningTokens: 9})
	require.Equal(t, 9, dst.ReasoningTokens)
}

func TestCopyOpenAIUsageFromResponsesUsage_ReasoningTokens(t *testing.T) {
	usage := copyOpenAIUsageFromResponsesUsage(&apicompat.ResponsesUsage{
		InputTokens:         10,
		OutputTokens:        12,
		OutputTokensDetails: &apicompat.ResponsesOutputTokensDetails{ReasoningTokens: 9},
	})
	require.Equal(t, 9, usage.ReasoningTokens)
	require.Equal(t, 12, usage.OutputTokens)

	require.Zero(t, copyOpenAIUsageFromResponsesUsage(&apicompat.ResponsesUsage{InputTokens: 1, OutputTokens: 2}).ReasoningTokens)
}

func TestOpenAIGatewayServiceRecordUsage_PersistsReasoningTokens(t *testing.T) {
	cases := []struct {
		name      string
		reasoning int
		wantNil   bool
	}{
		{name: "positive", reasoning: 321},
		{name: "zero stored as NULL", reasoning: 0, wantNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			subRepo := &openAIRecordUsageSubRepoStub{}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)

			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{
					RequestID: "resp_reasoning_tokens_" + tc.name,
					Model:     "gpt-5.4",
					Usage: OpenAIUsage{
						InputTokens:     20,
						OutputTokens:    400,
						ReasoningTokens: tc.reasoning,
					},
					Duration: time.Second,
				},
				APIKey:  &APIKey{ID: 10},
				User:    &User{ID: 20},
				Account: &Account{ID: 30},
			})

			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.Equal(t, 400, usageRepo.lastLog.OutputTokens, "billing output unchanged")
			if tc.wantNil {
				require.Nil(t, usageRepo.lastLog.ReasoningTokens)
				return
			}
			require.NotNil(t, usageRepo.lastLog.ReasoningTokens)
			require.Equal(t, tc.reasoning, *usageRepo.lastLog.ReasoningTokens)
		})
	}
}
