package regression

import (
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

func TestBug20_batch_checksum_idempotent_id(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	l := &model.RopewayLine{Code: "L-idem20", Name: "idempotent line", LengthM: 1400, TowerCount: 4, RatedSpeedMS: 5}
	if err := st.CreateLine(l); err != nil {
		t.Fatal(err)
	}
	sen := &model.RopeSensor{LineID: l.ID, Code: "W-IDEM", Kind: model.KindWind, Unit: "m/s", Enabled: true,
		Tolerance: 1, SoftMin: 0, SoftMax: 25, HardMin: -10, HardMax: 60}
	if err := st.CreateSensor(sen); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewManual(time.Now().UTC())
	svc := service.New(st, clk, cache.New(time.Minute), nil, nil, service.Params{})
	now := clk.Now()

	points := []model.TelemetryPointInput{
		{SensorCode: "W-IDEM", Seq: 1, TakenAt: now.Add(-30 * time.Second), Value: 6},
	}
	in := model.BatchInput{
		LineCode:    l.Code,
		WindowStart: now.Add(-5 * time.Minute),
		WindowEnd:   now,
		Checksum:    validation.ComputeChecksum(points),
		Points:      points,
	}
	first, err := svc.IngestBatch(in)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	second, err := svc.IngestBatch(in)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if second.BatchID != first.BatchID {
		t.Fatalf("identical checksum resubmission must reuse batch id %d, got %d", first.BatchID, second.BatchID)
	}
}
