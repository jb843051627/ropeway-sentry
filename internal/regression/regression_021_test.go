package regression

import (
	"testing"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func TestBug21_transitions_copy_isolated(t *testing.T) {
	want := []model.LineStatus{model.LineRestricted, model.LineMaintenance, model.LineClosed}

	first := model.AllowedTransitions(model.LineOpen)
	if len(first) != len(want) {
		t.Fatalf("open must expose %d successors, got %d", len(want), len(first))
	}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("successor[%d] = %s, want %s", i, first[i], want[i])
		}
	}

	first[0] = model.LineClosed
	second := model.AllowedTransitions(model.LineOpen)
	for i := range want {
		if second[i] != want[i] {
			t.Fatalf("mutating a previous result leaked into internal state at [%d]: got %s, want %s", i, second[i], want[i])
		}
	}
}
