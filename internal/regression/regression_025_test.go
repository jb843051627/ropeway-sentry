package regression

import (
	"math"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
)

func TestBug25_wire_rate_zero_span(t *testing.T) {
	now := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)

	single := []engine.WireSample{{At: now, Cumulative: 5}}
	rate, ok := engine.WireRate(single, time.Hour, now)
	if ok && math.IsNaN(rate) {
		t.Fatalf("zero-span window must not yield NaN rate, got %v", rate)
	}

	reset := []engine.WireSample{
		{At: now.Add(-time.Hour), Cumulative: 9},
		{At: now, Cumulative: 3},
	}
	r2, ok2 := engine.WireRate(reset, 2*time.Hour, now)
	if !ok2 || r2 != 0 || math.IsNaN(r2) {
		t.Fatalf("counter reset must fall back to (0,true), got (%v,%v)", r2, ok2)
	}
}
