package regression

import (
	"strings"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
)

func TestBug14_icing_relief_direction(t *testing.T) {
	p := engine.DefaultIcingPolicy()
	if p.IcingWindRelief != 1 {
		t.Fatalf("default icing relief should be 1, got %d", p.IcingWindRelief)
	}
	jan := p.ResolveForTime(time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC))
	if !jan.Active {
		t.Fatal("january must activate icing policy")
	}
	if got := jan.WindThresholds().RestrictedScale; got != 7 {
		t.Fatalf("active icing should lower restricted scale to 7, got %d", got)
	}
	desc := jan.Describe()
	if !strings.Contains(desc, "restricted<=7") {
		t.Fatalf("active icing summary must show tightened restricted scale, got %q", desc)
	}
}
