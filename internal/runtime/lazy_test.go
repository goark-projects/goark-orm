package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLazy_Load_whenConcurrent_shouldCallLoaderOnce(t *testing.T) {
	var calls atomic.Int64
	value := NewLazy(func(ctx context.Context) (int, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return 42, nil
	})
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := value.Load(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if got != 42 {
				errs <- fmt.Errorf("unexpected lazy value %d", got)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one loader call, got %d", calls.Load())
	}
}
