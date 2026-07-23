package service

import (
	"net/http"
	"testing"
	"time"
)

func TestClassifyBedrock429(t *testing.T) {
	cases := []struct {
		body string
		want bedrock429Kind
	}{
		{`{"message":"Too many tokens per day, please wait before trying again."}`, bedrock429DailyQuota},
		{`{"message":"Too many requests, please wait before trying again."}`, bedrock429RPMThrottle},
		{`{"message":"ThrottlingException: Rate exceeded"}`, bedrock429RPMThrottle},
		{`{"message":"some unknown error"}`, bedrock429RPMThrottle},
	}
	for _, c := range cases {
		if got := classifyBedrock429([]byte(c.body)); got != c.want {
			t.Errorf("classifyBedrock429(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}

func TestNextUTCMidnight(t *testing.T) {
	now := time.Date(2026, 7, 23, 8, 12, 0, 0, time.UTC)
	if got := nextUTCMidnight(now); !got.Equal(time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("nextUTCMidnight = %v, want 2026-07-24 00:00 UTC", got)
	}
	eom := time.Date(2026, 7, 31, 23, 59, 0, 0, time.UTC)
	if g := nextUTCMidnight(eom); !g.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("nextUTCMidnight(eom) = %v, want 2026-08-01", g)
	}
}

func TestBedrockCooldownUntil(t *testing.T) {
	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)

	if got := bedrockCooldownUntil(now, bedrock429DailyQuota, http.Header{}, 30, true); !got.Equal(time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("daily cooldown = %v, want next UTC midnight", got)
	}
	if off := bedrockCooldownUntil(now, bedrock429DailyQuota, http.Header{}, 30, false); !off.Equal(now.Add(30 * time.Second)) {
		t.Errorf("daily(disabled) = %v, want now+30s", off)
	}
	if rpm := bedrockCooldownUntil(now, bedrock429RPMThrottle, http.Header{}, 30, true); !rpm.Equal(now.Add(30 * time.Second)) {
		t.Errorf("rpm cooldown = %v, want now+30s", rpm)
	}
	h := http.Header{}
	h.Set("Retry-After", "45")
	if ra := bedrockCooldownUntil(now, bedrock429RPMThrottle, h, 30, true); !ra.Equal(now.Add(45 * time.Second)) {
		t.Errorf("rpm Retry-After = %v, want now+45s", ra)
	}
}
