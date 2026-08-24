package regression

import (
	"errors"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func TestBug23_tension_tolerance_precheck(t *testing.T) {
	ratio, err := engine.TensionRatio(80000, 80000, 0)
	if err == nil {
		t.Fatalf("zero tolerance must be rejected before division, got ratio=%v err=nil", ratio)
	}
	if !errors.Is(err, engine.ErrBadTolerance) {
		t.Fatalf("zero tolerance must map to ErrBadTolerance, got %v", err)
	}

	bad := &model.TensionBaseline{
		LineID: 1, SensorCode: "S-Z23", ExpectedN: 100, ToleranceN: 0,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if verr := bad.Validate(); verr == nil {
		t.Fatal("baseline with non-positive tolerance must fail validation")
	}
}
