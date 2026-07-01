package service

import (
	"context"
	"testing"
)

func TestBuildUpstreamRequestMantleAnthropic(t *testing.T) {
	s := &GatewayService{}
	acct := &Account{
		Platform:    PlatformAnthropic,
		Type:        AccountTypeBedrock,
		Credentials: map[string]any{"auth_mode": "bedrock_mantle", "aws_region": "eu-north-1", "api_key": "sk-mantle"},
	}
	body := []byte(`{"model":"global.anthropic.claude-opus-4-8-v1","messages":[]}`)
	req, _, err := s.buildUpstreamRequestAnthropicAPIKeyPassthrough(context.Background(), nil, acct, body, "sk-mantle")
	if err != nil {
		t.Fatalf("build err: %v", err)
	}
	if got := req.URL.String(); got != "https://bedrock-mantle.eu-north-1.api.aws/v1/messages" {
		t.Fatalf("url = %q", got)
	}
	if got := getHeaderRaw(req.Header, "x-api-key"); got != "sk-mantle" {
		t.Fatalf("x-api-key = %q", got)
	}
	if getHeaderRaw(req.Header, "anthropic-workspace-id") != "" {
		t.Fatal("mantle must not set anthropic-workspace-id")
	}
}
