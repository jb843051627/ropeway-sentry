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

func TestBug09_dedup_key_sensor_dim(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	l := &model.RopewayLine{Code: "L-dedup", Name: "dedup line", LengthM: 1500, TowerCount: 4, RatedSpeedMS: 5}
	if err := st.CreateLine(l); err != nil {
		t.Fatal(err)
	}
	mk := func(code string) *model.RopeSensor {
		return &model.RopeSensor{LineID: l.ID, Code: code, Kind: model.KindWind, Unit: "m/s", Enabled: true,
			Tolerance: 1, SoftMin: 0, SoftMax: 25, HardMin: -10, HardMax: 60}
	}
	for _, sen := range []*model.RopeSensor{mk("W-A"), mk("W-B")} {
		if err := st.CreateSensor(sen); err != nil {
			t.Fatal(err)
		}
	}
	clk := clock.NewManual(time.Now().UTC())
	svc := service.New(st, clk, cache.New(time.Minute), nil, nil, service.Params{})
	now := clk.Now()
	points := []model.TelemetryPointInput{
		{SensorCode: "W-A", Seq: 1, TakenAt: now.Add(-30 * time.Second), Value: 29},
		{SensorCode: "W-B", Seq: 1, TakenAt: now.Add(-30 * time.Second), Value: 29},
	}
	in := model.BatchInput{
		LineCode:    l.Code,
		WindowStart: now.Add(-5 * time.Minute),
		WindowEnd:   now,
		Checksum:    validation.ComputeChecksum(points),
		Points:      points,
	}
	res, err := svc.IngestBatch(in)
	if err != nil {
		t.Fatalf("ingest batch: %v", err)
	}
	if len(res.AlertsNew) != 2 {
		t.Fatalf("two sensors must raise two independent alerts, got %d (%v)", len(res.AlertsNew), res.AlertsNew)
	}
	opens, err := st.ListAlerts("open", 100)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, al := range opens {
		if al.Kind == "wind_critical" && al.SensorID > 0 {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 per-sensor wind_critical alerts, got %d", n)
	}
}
