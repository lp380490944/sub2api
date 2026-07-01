package service

import "testing"

func TestBuildBedrockMantleURLs(t *testing.T) {
	if got := BuildBedrockMantleBaseURL("eu-north-1"); got != "https://bedrock-mantle.eu-north-1.api.aws" {
		t.Fatalf("base url = %q", got)
	}
	if got := BuildBedrockMantleBaseURL(""); got != "https://bedrock-mantle.eu-north-1.api.aws" {
		t.Fatalf("empty-region base url = %q", got)
	}
	if got := BuildBedrockMantleMessagesURL("eu-north-1"); got != "https://bedrock-mantle.eu-north-1.api.aws/v1/messages" {
		t.Fatalf("messages url = %q", got)
	}
	if got := BuildBedrockMantleChatCompletionsURL("eu-north-1"); got != "https://bedrock-mantle.eu-north-1.api.aws/v1/chat/completions" {
		t.Fatalf("cc url = %q", got)
	}
}

func TestBedrockMantleRegion(t *testing.T) {
	if got := bedrockMantleRegion(nil); got != "eu-north-1" {
		t.Fatalf("nil region = %q", got)
	}
	acct := &Account{Credentials: map[string]any{}}
	if got := bedrockMantleRegion(acct); got != "eu-north-1" {
		t.Fatalf("default region = %q", got)
	}
	acct.Credentials["aws_region"] = "us-east-1"
	if got := bedrockMantleRegion(acct); got != "us-east-1" {
		t.Fatalf("aws_region = %q", got)
	}
}

func TestResolveBedrockMantleModelID(t *testing.T) {
	acct := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{}}
	cases := map[string]string{
		"claude-opus-4-8": "global.anthropic.claude-opus-4-8-v1",
		"claude-opus-4-7": "global.anthropic.claude-opus-4-7-v1",
	}
	for in, want := range cases {
		got, ok := ResolveBedrockMantleModelID(acct, in)
		if !ok || got != want {
			t.Fatalf("resolve(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := ResolveBedrockMantleModelID(acct, "definitely-not-a-model"); ok {
		t.Fatalf("expected unknown model to return ok=false")
	}
}
