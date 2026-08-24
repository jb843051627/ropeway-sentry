package regression

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/ingest"
)

func TestBug04_retry_cancel_precheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- ingest.Retry(ctx, 8, func() error {
			calls++
			return errors.New("boom")
		})
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v (fn calls=%d)", err, calls)
		}
		if calls != 0 {
			t.Fatalf("fn ran %d times on cancelled ctx", calls)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Retry ignored cancellation")
	}
}
