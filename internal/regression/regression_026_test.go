package regression

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/clock"
	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/service"
	"github.com/jb843051627/ropeway-sentry/internal/store"
)

func TestBug26_kpi_close_rate_zero_opened(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	l := &model.RopewayLine{Code: "L-kpi26", Name: "kpi line", LengthM: 1500, TowerCount: 4, RatedSpeedMS: 5}
	if err := st.CreateLine(l); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewManual(time.Now().UTC())
	svc := service.New(st, clk, cache.New(time.Minute), nil, nil, service.Params{})

	rep, err := svc.LineWeeklyKPI(l.ID, 7)
	if err != nil {
		t.Fatalf("weekly kpi: %v", err)
	}
	if math.IsNaN(rep.CloseRate) || math.IsInf(rep.CloseRate, 0) {
		t.Fatalf("close rate with zero opened alerts must stay finite, got %v", rep.CloseRate)
	}
	for _, b := range rep.Buckets {
		if math.IsNaN(b.AlertCloseRate) || math.IsInf(b.AlertCloseRate, 0) ||
			math.IsNaN(b.IntegrityRate) || math.IsInf(b.IntegrityRate, 0) {
			t.Fatalf("day %s rates must stay finite, got close=%v integrity=%v", b.Date, b.AlertCloseRate, b.IntegrityRate)
		}
	}
}
