package nikologs

import (
	"fmt"
	"time"
)

const maxRetries = 3

// flushLoop runs in a background goroutine, batching and flushing log entries.
// It flushes when the batch reaches batchSize or the flush interval timer fires.
func (c *Client) flushLoop() {
	defer close(c.done)

	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	batch := make([]*entry, 0, c.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toSend := batch
		batch = make([]*entry, 0, c.batchSize)
		c.sendWithRetry(toSend)
	}

	for {
		select {
		case e := <-c.entries:
			batch = append(batch, e)
			if len(batch) >= c.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-c.quit:
			// Drain remaining entries from channel
			for {
				select {
				case e := <-c.entries:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

// sendWithRetry sends a batch with exponential backoff retry.
func (c *Client) sendWithRetry(entries []*entry) {
	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := c.sendBatch(entries)
		if err == nil {
			return
		}
		if attempt < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		} else {
			c.onError(fmt.Errorf("nikologs: flush failed after %d retries: %w", maxRetries, err))
		}
	}
}

// Flush manually flushes any buffered log entries.
func (c *Client) Flush() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		batch := make([]*entry, 0, c.batchSize)
		for {
			select {
			case e := <-c.entries:
				batch = append(batch, e)
			default:
				if len(batch) > 0 {
					c.sendWithRetry(batch)
				}
				return
			}
		}
	}()
	<-done
}
