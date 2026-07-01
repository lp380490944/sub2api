package service

import "testing"

func TestMantleOpenAIChatCompletionsURL(t *testing.T) {
	acct := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"auth_mode": "bedrock_mantle", "aws_region": "eu-north-1"},
	}
	got := buildOpenAIChatCompletionsURL(acct.GetOpenAIBaseURL())
	if got != "https://bedrock-mantle.eu-north-1.api.aws/v1/chat/completions" {
		t.Fatalf("mantle cc url = %q", got)
	}
}
