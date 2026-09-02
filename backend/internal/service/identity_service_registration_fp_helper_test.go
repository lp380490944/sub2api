package service

import "context"

// trackingIdentityCache counts Get/Set calls and stores the last-written fingerprint.
// Used to assert that non-claude-cli UAs never touch the cache.
type trackingIdentityCache struct {
	stored     *Fingerprint
	getCalls   int
	setCalls   int
	initialFP  *Fingerprint
	initialErr error
}

func (c *trackingIdentityCache) GetFingerprint(_ context.Context, _ int64) (*Fingerprint, error) {
	c.getCalls++
	if c.stored != nil {
		return c.stored, nil
	}
	return c.initialFP, c.initialErr
}

func (c *trackingIdentityCache) SetFingerprint(_ context.Context, _ int64, fp *Fingerprint) error {
	c.setCalls++
	cp := *fp
	c.stored = &cp
	return nil
}

func (c *trackingIdentityCache) GetMaskedSessionID(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (c *trackingIdentityCache) SetMaskedSessionID(_ context.Context, _ int64, _ string) error {
	return nil
}
