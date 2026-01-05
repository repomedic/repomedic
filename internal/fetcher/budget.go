package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RequestBudget struct {
	mu        sync.Mutex
	remaining int
	reset     time.Time
	now       func() time.Time
	probed    bool
	cooldown  time.Time
	notifyCh  chan struct{}
}

func NewRequestBudget() *RequestBudget {
	b := &RequestBudget{
		remaining: 5000, // Default conservative start
		reset:     time.Now().Add(1 * time.Hour),
		now:       time.Now,
		notifyCh:  make(chan struct{}),
	}
	return b
}

func (b *RequestBudget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.remaining
}

func (b *RequestBudget) Acquire(ctx context.Context, n int) error {
	if ctx == nil {
		return fmt.Errorf("Acquire: nil context")
	}
	if n <= 0 {
		return fmt.Errorf("Acquire: n must be > 0 (got %d)", n)
	}
	if b == nil {
		return fmt.Errorf("Acquire: nil RequestBudget")
	}
	if b.now == nil {
		return fmt.Errorf("Acquire: RequestBudget.now is nil (use NewRequestBudget)")
	}
	if b.notifyCh == nil {
		return fmt.Errorf("Acquire: RequestBudget.notifyCh is nil (use NewRequestBudget)")
	}

	for i := 0; i < n; i++ {
		if err := b.acquireOne(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *RequestBudget) acquireOne(ctx context.Context) error {
	for {
		b.mu.Lock()
		now := b.now()

		if now.Before(b.cooldown) {
			until := b.cooldown
			ch := b.notifyCh
			b.mu.Unlock()

			wait := until.Sub(now)
			if wait < 0 {
				wait = 0
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-ch:
				if !timer.Stop() {
					<-timer.C
				}
				continue
			case <-timer.C:
				continue
			}
		}

		if b.remaining > 0 {
			b.remaining--
			b.mu.Unlock()
			return nil
		}

		// If reset has passed but we haven't observed a refreshed budget yet,
		// allow exactly one probe request and then block until UpdateFromResponse.
		if !now.Before(b.reset) {
			if !b.probed {
				b.probed = true
				b.mu.Unlock()
				return nil
			}
			ch := b.notifyCh
			b.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ch:
				continue
			}
		}

		// Wait until reset time or until UpdateFromResponse signals budget changes.
		reset := b.reset
		ch := b.notifyCh
		b.mu.Unlock()

		wait := reset.Sub(now)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-ch:
			if !timer.Stop() {
				<-timer.C
			}
			continue
		case <-timer.C:
			continue
		}
	}
}

func (b *RequestBudget) signalLocked() {
	if b.notifyCh == nil {
		b.notifyCh = make(chan struct{})
		return
	}
	close(b.notifyCh)
	b.notifyCh = make(chan struct{})
}

func (b *RequestBudget) UpdateFromResponse(resp *http.Response) {
	if resp == nil {
		return
	}
	if b == nil {
		return
	}
	if b.now == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	changed := false

	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			if seconds > 0 {
				until := b.now().Add(time.Duration(seconds) * time.Second)
				if until.After(b.cooldown) {
					b.cooldown = until
					changed = true
				}
			}
		}
	}

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			if val >= 0 {
				if b.remaining != val {
					b.remaining = val
					changed = true
				}
			}
		}
	}

	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			if val > 0 {
				newReset := time.Unix(val, 0)
				if !b.reset.Equal(newReset) {
					b.reset = newReset
					changed = true
				}
			}
		}
	}

	if changed {
		b.probed = false
		b.signalLocked()
	}
}
