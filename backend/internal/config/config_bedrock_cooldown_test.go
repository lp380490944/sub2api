package config

import "testing"

func TestBedrockCooldownDefaults(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Gateway.BedrockRPMCooldownSec != 30 {
		t.Errorf("BedrockRPMCooldownSec = %d, want 30", cfg.Gateway.BedrockRPMCooldownSec)
	}
	if !cfg.Gateway.BedrockDailyQuotaCooldownEnabled {
		t.Error("BedrockDailyQuotaCooldownEnabled = false, want true")
	}
	if cfg.Gateway.BedrockFailureThreshold != 2 {
		t.Errorf("BedrockFailureThreshold = %d, want 2", cfg.Gateway.BedrockFailureThreshold)
	}
	if cfg.Gateway.BedrockFailureWindowSec != 300 {
		t.Errorf("BedrockFailureWindowSec = %d, want 300", cfg.Gateway.BedrockFailureWindowSec)
	}
	if cfg.Gateway.BedrockFailureCooldownSec != 600 {
		t.Errorf("BedrockFailureCooldownSec = %d, want 600", cfg.Gateway.BedrockFailureCooldownSec)
	}
}
