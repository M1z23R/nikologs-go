package nikologs

// flushLoop runs in a background goroutine, batching and flushing log entries.
func (c *Client) flushLoop() {
	defer close(c.done)
	// Full implementation in Task 7.
	for {
		select {
		case <-c.quit:
			return
		case <-c.entries:
			// drain, will be replaced
		}
	}
}
