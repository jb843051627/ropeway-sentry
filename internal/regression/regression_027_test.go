package regression

import (
	"math"
	"testing"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func TestBug27_integrity_rate_zero_total(t *testing.T) {
	var tally model.QualityTally
	got := tally.IntegrityRate()
	if got != 1 {
		t.Fatalf("empty tally integrity rate must be 1 (no penalty), got %v", got)
	}
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("empty tally integrity rate must not be NaN/Inf, got %v", got)
	}
}
