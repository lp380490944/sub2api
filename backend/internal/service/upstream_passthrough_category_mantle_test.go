package service

import "testing"

func TestDeriveUpstreamCategoryMantleIsRelay(t *testing.T) {
	a := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "bedrock_mantle"}}
	if got := DeriveUpstreamCategory(a); got != CategoryRelay {
		t.Fatalf("mantle category = %v; want relay", got)
	}
}

func TestDeriveUpstreamCategorySigv4BedrockUnaffected(t *testing.T) {
	a := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "sigv4"}}
	if got := DeriveUpstreamCategory(a); got != CategoryOfficial {
		t.Fatalf("sigv4 bedrock category = %v; want official", got)
	}
}
