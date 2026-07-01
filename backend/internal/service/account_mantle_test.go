package service

import "testing"

func TestIsBedrockMantle(t *testing.T) {
	a := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "bedrock_mantle"}}
	if !a.IsBedrockMantle() {
		t.Fatal("expected anthropic bedrock+bedrock_mantle to be IsBedrockMantle")
	}
	sig := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "sigv4"}}
	if sig.IsBedrockMantle() {
		t.Fatal("sigv4 bedrock must not be IsBedrockMantle")
	}
	oa := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"auth_mode": "bedrock_mantle"}}
	if oa.IsBedrockMantle() {
		t.Fatal("openai account must not be IsBedrockMantle")
	}
}

func TestIsOpenAIBedrockMantle(t *testing.T) {
	oa := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"auth_mode": "bedrock_mantle"}}
	if !oa.IsOpenAIBedrockMantle() {
		t.Fatal("expected openai apikey+bedrock_mantle to be IsOpenAIBedrockMantle")
	}
	plain := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://x"}}
	if plain.IsOpenAIBedrockMantle() {
		t.Fatal("plain openai apikey must not be IsOpenAIBedrockMantle")
	}
}

func TestGetOpenAIBaseURLMantle(t *testing.T) {
	oa := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"auth_mode": "bedrock_mantle", "aws_region": "eu-north-1", "base_url": "https://ignored"}}
	if got := oa.GetOpenAIBaseURL(); got != "https://bedrock-mantle.eu-north-1.api.aws" {
		t.Fatalf("mantle base url = %q", got)
	}
}
