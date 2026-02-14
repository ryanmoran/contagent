package apple

import "time"

// SetReadyCheckTiming configures the readiness check parameters for testing.
func (c *Container) SetReadyCheckTiming(retries int, baseDelay time.Duration) {
	c.readyRetries = retries
	c.readyBaseDelay = baseDelay
}
