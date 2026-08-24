package regression

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/clock"
	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/service"
	"github.com/jb843051627/ropeway-sentry/internal/store"
	"github.com/jb843051627/ropeway-sentry/internal/validation"
)

func TestBug05_error_wrap_chain(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	clk := clock.NewManual(time.Now().UTC())
	svc := service.New(st, clk, cache.New(time.Minute), nil, nil, service.Params{})

	points := []model.TelemetryPointInput{{SensorCode: "S-x", Seq: 1, TakenAt: clk.Now(), Value: 1}}
	in := model.BatchInput{
		LineCode:    "L-nope",
		WindowStart: clk.Now().Add(-time.Minute),
		WindowEnd:   clk.Now(),
		Checksum:    validation.ComputeChecksum(points),
		Points:      points,
	}
	if _, err := svc.IngestBatch(in); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown line should preserve ErrNotFound chain, got %v", err)
	}

	bl := &model.TensionBaseline{
		LineID: 999, SensorCode: "ghost", ExpectedN: 100, ToleranceN: 5,
		ValidFrom: clk.Now().Add(-time.Hour), ValidTo: clk.Now().Add(time.Hour),
	}
	if _, err := svc.UpsertBaseline(bl); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown baseline sensor should preserve ErrNotFound chain, got %v", err)
	}
}
