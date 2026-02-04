package backend

import (
	"time"
)

// reconnect attempts to reconnect to backend
func (c *Client) reconnect() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.mu.Lock()
		c.reconnCount++
		count := c.reconnCount
		c.mu.Unlock()

		// Check max reconnection attempts
		if c.config.MaxReconnect > 0 && count > c.config.MaxReconnect {
			c.logger.Errorf("Max reconnection attempts (%d) reached, giving up", c.config.MaxReconnect)
			return
		}

		c.logger.Infof("Attempting to reconnect (attempt %d)...", count)

		// Wait before reconnecting (with context cancellation support)
		select {
		case <-time.After(c.config.ReconnectDelay):
			// Continue to reconnect
		case <-c.ctx.Done():
			c.logger.Info("Reconnection cancelled due to context cancellation")
			return
		}

		// Try to connect
		if err := c.connect(); err != nil {
			c.logger.Errorf("Reconnection failed: %v", err)
			continue
		}

		// Successfully reconnected, start pumps
		c.Run()
		return
	}
}

// SetReconnectDelay sets the delay between reconnection attempts
func (c *Client) SetReconnectDelay(delay time.Duration) {
	c.config.ReconnectDelay = delay
}

// SetMaxReconnect sets the maximum number of reconnection attempts
func (c *Client) SetMaxReconnect(max int) {
	c.config.MaxReconnect = max
}

// ResetReconnectCount resets the reconnection counter
func (c *Client) ResetReconnectCount() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnCount = 0
}

// GetReconnectCount returns the current reconnection count
func (c *Client) GetReconnectCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnCount
}
