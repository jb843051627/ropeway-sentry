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

func TestBug18_all_points_outside_rejected(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	l := &model.RopewayLine{Code: "L-out18", Name: "outside line", LengthM: 1200, TowerCount: 4, RatedSpeedMS: 5}
	if err := st.CreateLine(l); err != nil {
		t.Fatal(err)
	}
	sen := &model.RopeSensor{LineID: l.ID, Code: "W-OUT", Kind: model.KindWind, Unit: "m/s", Enabled: true,
		Tolerance: 1, SoftMin: 0, SoftMax: 25, HardMin: -10, HardMax: 60}
	if err := st.CreateSensor(sen); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewManual(time.Now().UTC())
	svc := service.New(st, clk, cache.New(time.Minute), nil, nil, service.Params{})
	now := clk.Now()

	points := []model.TelemetryPointInput{
		{SensorCode: "W-OUT", Seq: 1, TakenAt: now.Add(-30 * time.Minute), Value: 5},
	}
	in := model.BatchInput{
		LineCode:    l.Code,
		WindowStart: now.Add(-5 * time.Minute),
		WindowEnd:   now,
		Checksum:    validation.ComputeChecksum(points),
		Points:      points,
	}
	res, err := svc.IngestBatch(in)
	if !errors.Is(err, model.ErrEmptyBatch) {
		t.Fatalf("batch fully outside declared window must be rejected with ErrEmptyBatch, got err=%v res=%+v", err, res)
	}
}
