package fetcher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleFlight(t *testing.T) {
	var g Group
	var calls int32

	fn := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(100 * time.Millisecond)
		return "result", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err, _ := g.Do("key", fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
			}
			if val != "result" {
				t.Errorf("got %v, want %v", val, "result")
			}
		}()
	}

	wg.Wait()

	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
}
