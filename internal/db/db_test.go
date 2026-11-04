package db

import (
	"errors"
	"testing"
)

func TestQueriesOnUnconnectedDBReturnError(t *testing.T) {
	for name, d := range map[string]*DB{"nil": nil, "zero": {}} {
		if _, err := d.GetActiveSessions(); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s: GetActiveSessions err = %v", name, err)
		}
		if _, err := d.GetExecutionPlan("x"); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s: GetExecutionPlan err = %v", name, err)
		}
		if _, err := d.GetSQLStats("x"); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s: GetSQLStats err = %v", name, err)
		}
	}
}
