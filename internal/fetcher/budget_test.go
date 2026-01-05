package fetcher

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRequestBudget(t *testing.T) {
	fixedNow := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	getRemaining := func(t *testing.T, b *RequestBudget) int {
		t.Helper()
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.remaining
	}

	getReset := func(t *testing.T, b *RequestBudget) time.Time {
		t.Helper()
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.reset
	}

	setState := func(t *testing.T, b *RequestBudget, remaining int, reset time.Time) {
		t.Helper()
		b.mu.Lock()
		b.remaining = remaining
		b.reset = reset
		b.mu.Unlock()
	}

	t.Run("Acquire ok", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }

		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
	})

	t.Run("UpdateFromResponse sets remaining and reset", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }

		resp := &http.Response{Header: make(http.Header)}
		resp.Header.Set("X-RateLimit-Remaining", "10")
		resp.Header.Set("X-RateLimit-Reset", "1700000000")

		b.UpdateFromResponse(resp)

		if rem := getRemaining(t, b); rem != 10 {
			t.Fatalf("Expected 10 remaining, got %d", rem)
		}
		if r := getReset(t, b); !r.Equal(time.Unix(1700000000, 0)) {
			t.Fatalf("Expected reset %v, got %v", time.Unix(1700000000, 0), r)
		}
	})

	t.Run("Retry-After causes cooldown blocking", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 5000, fixedNow.Add(-1*time.Hour))

		resp := &http.Response{Header: make(http.Header)}
		resp.Header.Set("Retry-After", "60")
		b.UpdateFromResponse(resp)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := b.Acquire(ctx, 1); err == nil {
			t.Fatalf("Expected context deadline exceeded during cooldown")
		}
	})

	t.Run("Retry-After extends cooldown", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 5000, fixedNow.Add(-1*time.Hour))

		resp1 := &http.Response{Header: make(http.Header)}
		resp1.Header.Set("Retry-After", "10")
		b.UpdateFromResponse(resp1)

		resp2 := &http.Response{Header: make(http.Header)}
		resp2.Header.Set("Retry-After", "60")
		b.UpdateFromResponse(resp2)

		b.mu.Lock()
		cooldown := b.cooldown
		b.mu.Unlock()
		if !cooldown.Equal(fixedNow.Add(60 * time.Second)) {
			t.Fatalf("Expected cooldown %v, got %v", fixedNow.Add(60*time.Second), cooldown)
		}
	})

	t.Run("UpdateFromResponse ignores invalid headers", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 7, time.Unix(123, 0))

		resp := &http.Response{Header: make(http.Header)}
		resp.Header.Set("X-RateLimit-Remaining", "nope")
		resp.Header.Set("X-RateLimit-Reset", "not-a-time")

		b.UpdateFromResponse(resp)

		if rem := getRemaining(t, b); rem != 7 {
			t.Fatalf("Expected remaining to stay 7, got %d", rem)
		}
		if r := getReset(t, b); !r.Equal(time.Unix(123, 0)) {
			t.Fatalf("Expected reset to stay %v, got %v", time.Unix(123, 0), r)
		}
	})

	t.Run("Exhausted before reset returns error", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 0, fixedNow.Add(1*time.Hour))

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := b.Acquire(ctx, 1); err == nil {
			t.Fatalf("Expected context deadline exceeded")
		}
	})

	t.Run("After reset allows one probe request", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 0, fixedNow.Add(-1*time.Second))

		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("Expected probe Acquire to succeed, got error: %v", err)
		}
		if rem := getRemaining(t, b); rem != 0 {
			t.Fatalf("Expected remaining 0 after probe, got %d", rem)
		}
	})

	t.Run("After reset only allows one probe until update", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 0, fixedNow.Add(-1*time.Second))

		if err := b.Acquire(context.Background(), 1); err != nil {
			t.Fatalf("Expected first probe to succeed, got %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := b.Acquire(ctx, 1); err == nil {
			t.Fatalf("Expected second acquire to block until context deadline")
		}
	})

	t.Run("UpdateFromResponse wakes waiters", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }
		setState(t, b, 0, fixedNow.Add(1*time.Hour))

		errCh := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			errCh <- b.Acquire(ctx, 1)
		}()

		time.Sleep(10 * time.Millisecond)
		resp := &http.Response{Header: make(http.Header)}
		resp.Header.Set("X-RateLimit-Remaining", "1")
		resp.Header.Set("X-RateLimit-Reset", "1700000000")
		b.UpdateFromResponse(resp)

		if err := <-errCh; err != nil {
			t.Fatalf("Expected Acquire to succeed after update, got %v", err)
		}
	})

	t.Run("Invalid inputs fail fast", func(t *testing.T) {
		b := NewRequestBudget()
		b.now = func() time.Time { return fixedNow }

		tests := []struct {
			name string
			ctx  context.Context
			n    int
		}{
			{name: "nil ctx", ctx: nil, n: 1},
			{name: "n=0", ctx: context.Background(), n: 0},
			{name: "n<0", ctx: context.Background(), n: -1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := b.Acquire(tt.ctx, tt.n); err == nil {
					t.Fatalf("Expected error")
				}
			})
		}
	})
}
