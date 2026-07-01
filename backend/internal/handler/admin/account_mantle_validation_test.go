package admin

import "testing"

func TestNormalizeBedrockMantleCredentials(t *testing.T) {
	// Missing api_key -> error
	if err := normalizeBedrockMantleCredentials(map[string]any{"auth_mode": "bedrock_mantle"}); err == nil {
		t.Fatal("expected error for missing api_key")
	}
	// Default region filled
	creds := map[string]any{"auth_mode": "bedrock_mantle", "api_key": "sk"}
	if err := normalizeBedrockMantleCredentials(creds); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if creds["aws_region"] != "eu-north-1" {
		t.Fatalf("aws_region = %v; want eu-north-1", creds["aws_region"])
	}
	// Non-mantle untouched
	other := map[string]any{"auth_mode": "sigv4"}
	if err := normalizeBedrockMantleCredentials(other); err != nil {
		t.Fatalf("non-mantle should be no-op, got %v", err)
	}
}
