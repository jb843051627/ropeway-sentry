package regression

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/store"
)

func TestBug06_require_affected_zero(t *testing.T) {
	st, stErr := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if stErr != nil {
		t.Fatal(stErr)
	}
	defer st.Close()
	now := time.Now().UTC()
	if err := st.AckAlert(987654, "op", now); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("AckAlert on missing id should be ErrConflict, got %v", err)
	}
	if err := st.SetSensorEnabled(987654, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("SetSensorEnabled on missing id should be ErrNotFound, got %v", err)
	}
}
