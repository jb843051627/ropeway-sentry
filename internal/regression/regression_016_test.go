package regression

import (
	"errors"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/validation"
)

func TestBug16_window_future_sentinel(t *testing.T) {
	now := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)
	rule := validation.DefaultWindowRule()

	err := validation.CheckWindow(now.Add(-time.Minute), now.Add(time.Minute), now, rule)
	if !errors.Is(err, model.ErrFutureWindow) {
		t.Fatalf("window ending after now must fail with ErrFutureWindow, got %v", err)
	}

	overSpan := validation.CheckWindow(now.Add(-8*time.Hour), now.Add(-time.Hour), now, rule)
	if !errors.Is(overSpan, model.ErrExpiredWindow) {
		t.Fatalf("over-span past window must keep ErrExpiredWindow sentinel, got %v", overSpan)
	}
}
