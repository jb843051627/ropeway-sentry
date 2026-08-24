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
)

func TestBug07_double_ack_conflict(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	l := &model.RopewayLine{Code: "L-ack", Name: "ack line", LengthM: 1200, TowerCount: 4, RatedSpeedMS: 5}
	if err := st.CreateLine(l); err != nil {
		t.Fatal(err)
	}
	sen := &model.RopeSensor{LineID: l.ID, Code: "W-A7", Kind: model.KindWind, Unit: "m/s", Enabled: true,
		Tolerance: 1, SoftMin: 0, SoftMax: 25, HardMin: -10, HardMax: 60}
	if err := st.CreateSensor(sen); err != nil {
		t.Fatal(err)
	}
	a := &model.Alert{LineID: l.ID, SensorID: sen.ID, DedupKey: "L-ack|wind_restricted",
		Kind: "wind_restricted", Severity: model.SeverityWarning, Message: "gust"}
	if err := st.InsertAlert(a); err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, clock.NewManual(time.Now().UTC()), cache.New(time.Minute), nil, nil, service.Params{})
	if _, err := svc.AckAlert(a.ID, "alice"); err != nil {
		t.Fatalf("first ack failed: %v", err)
	}
	if _, err := svc.AckAlert(a.ID, "bob"); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("second ack must conflict, got %v", err)
	}
}
