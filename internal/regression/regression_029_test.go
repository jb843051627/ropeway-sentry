package regression

import (
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
)

func TestBug29_winter_margin_direction(t *testing.T) {
	jan := engine.DefaultIcingPolicy().ResolveForTime(time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC))
	if !jan.Active {
		t.Fatal("january must activate icing policy")
	}

	base := engine.FrostInput{TempC: -2, HumidityPct: 95, WindMS: 1}
	plain := engine.EvaluateFrost(base, 1.0)
	winter := engine.EvaluateFrost(base, engine.WinterFrostMargin)
	if winter.Score <= plain.Score {
		t.Fatalf("winter margin must amplify frost score, got winter=%v plain=%v", winter.Score, plain.Score)
	}

	if got := jan.TiltLimit(2.0); got >= 2.0 {
		t.Fatalf("icing tilt limit must tighten below base limit, got %v", got)
	}
}
