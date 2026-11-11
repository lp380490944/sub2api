package openai_ws_v2

import (
	"context"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// WS passthrough must surface upstream reasoning token counts per turn (for
// usage_logs.reasoning_tokens) without changing billed output tokens.
func TestRelay_ReasoningTokensPerTurn(t *testing.T) {
	t.Parallel()

	clientConn := newPassthroughTestFrameConn(nil, false)
	upstreamConn := newPassthroughTestFrameConn([]passthroughTestFrame{
		{msgType: coderws.MessageText, payload: []byte(`{"type":"response.completed","response":{"id":"resp_r1","usage":{"input_tokens":100,"output_tokens":60,"total_tokens":160,"output_tokens_details":{"reasoning_tokens":40}}}}`)},
		{msgType: coderws.MessageText, payload: []byte(`{"type":"response.completed","response":{"id":"resp_r2","usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18,"completion_tokens_details":{"reasoning_tokens":5}}}}`)},
		{msgType: coderws.MessageText, payload: []byte(`{"type":"response.completed","response":{"id":"resp_r3","usage":{"input_tokens":3,"output_tokens":4}}}`)},
	}, true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var turns []RelayTurnResult
	result, relayExit := Relay(ctx, clientConn, upstreamConn,
		[]byte(`{"type":"response.create","model":"gpt-5.4","input":[]}`),
		RelayOptions{OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) }})
	require.Nil(t, relayExit)
	require.Len(t, turns, 3)

	require.Equal(t, 40, turns[0].Usage.ReasoningTokens)
	require.Equal(t, 60, turns[0].Usage.OutputTokens, "OpenAI folds reasoning into output; billed output unchanged")
	require.Equal(t, 5, turns[1].Usage.ReasoningTokens)
	require.Equal(t, 8, turns[1].Usage.OutputTokens)
	require.Zero(t, turns[2].Usage.ReasoningTokens, "missing reasoning details must stay zero")

	require.Equal(t, 45, result.Usage.ReasoningTokens)
	require.Equal(t, 72, result.Usage.OutputTokens)
}
