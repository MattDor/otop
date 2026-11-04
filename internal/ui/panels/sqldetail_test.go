package panels

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
)

type fakeSQL struct {
	mu       sync.Mutex
	plans    map[string][]models.PlanRow
	stats    map[string]*models.SQLStats
	planErr  error
	statsErr error
	calls    []string // SQL IDs passed to GetExecutionPlan
}

func (f *fakeSQL) GetExecutionPlan(sqlID string) ([]models.PlanRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sqlID)
	return f.plans[sqlID], f.planErr
}

func (f *fakeSQL) GetSQLStats(sqlID string) (*models.SQLStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stats[sqlID], f.statsErr
}

func (f *fakeSQL) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newFakeSQL() *fakeSQL {
	return &fakeSQL{
		plans: map[string][]models.PlanRow{
			"sql1": {
				{ID: 0, Depth: 0, Operation: "SELECT STATEMENT", Cost: 2},
				{ID: 1, ParentID: 0, Depth: 1, Operation: "TABLE ACCESS", Options: "FULL", ObjectName: "DUAL", Cardinality: 1, Bytes: 2, Cost: 2},
			},
		},
		stats: map[string]*models.SQLStats{
			"sql1": {SQLID: "sql1", SQLText: "select 1 from dual", Executions: 4, ElapsedTimeMicros: 2_000_000, CPUTimeMicros: 500_000, BufferGets: 12, DiskReads: 3, Rows: 4},
			"sql3": {SQLID: "sql3", SQLText: "update t set x = 1", Executions: 1},
		},
	}
}

func setupDetail(t *testing.T, src sqlSource) (*SQLDetailPanel, *tviewApp) {
	t.Helper()
	p := newSQLDetail(nil, src)
	app, screen := startApp(t, p.text)
	p.app = app
	return p, &tviewApp{app, screen}
}

func (p *SQLDetailPanel) plainText() string { return p.text.GetText(true) }

func TestSQLDetail_ShowsSessionStatsAndPlan(t *testing.T) {
	src := newFakeSQL()
	p, a := setupDetail(t, src)

	s := threeSessions[0]
	s.Program, s.Machine, s.WaitEvent, s.WaitSeconds = "sqlplus@host", "host1", "db file sequential read", 7
	onMain(t, a.app, func() { p.OnContext(uictx.SessionContext{Session: s}) })
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "Execution Plan") }, "rendered")

	var text string
	onMain(t, a.app, func() { text = p.plainText() })
	for _, want := range []string{
		"SID,Serial#:   1,10",
		"Username:      SCOTT",
		"Program:       sqlplus@host",
		"Wait Event:    db file sequential read (7 s)",
		"SQL ID: sql1",
		"select 1 from dual",
		"Executions:    4",
		"Elapsed:       2.000 s",
		"CPU:           0.500 s",
		"Elapsed/exec:  500.000 ms",
		"Buffer Gets:   12",
		"TABLE ACCESS FULL DUAL",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text missing %q\n--- got ---\n%s", want, text)
		}
	}
}

func TestSQLDetail_SessionWithoutSQLSkipsQueries(t *testing.T) {
	src := newFakeSQL()
	p, a := setupDetail(t, src)

	onMain(t, a.app, func() { p.OnContext(uictx.SessionContext{Session: threeSessions[1]}) })
	onMain(t, a.app, func() {
		text := p.plainText()
		if !strings.Contains(text, "Username:      HR") {
			t.Errorf("session info missing:\n%s", text)
		}
		if !strings.Contains(text, "No SQL statement") {
			t.Errorf("missing 'no SQL' note:\n%s", text)
		}
	})
	if n := src.callCount(); n != 0 {
		t.Errorf("DB queried %d times, want 0", n)
	}
}

func TestSQLDetail_SQLContextForSameStatementIsDeduplicated(t *testing.T) {
	src := newFakeSQL()
	p, a := setupDetail(t, src)

	s := threeSessions[0]
	onMain(t, a.app, func() {
		// This is exactly what the session list emits on selection.
		p.OnContext(uictx.SessionContext{Session: s})
		p.OnContext(uictx.SQLContext{SQLID: s.SQLID, SQLText: s.SQLText})
	})
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "Execution Plan") }, "rendered")
	if n := src.callCount(); n != 1 {
		t.Errorf("GetExecutionPlan called %d times, want 1", n)
	}
	onMain(t, a.app, func() {
		if !strings.Contains(p.plainText(), "Session:") {
			t.Error("session section was dropped by the duplicate SQLContext")
		}
	})
}

func TestSQLDetail_SQLContextForOtherStatementShowsSQLOnly(t *testing.T) {
	src := newFakeSQL()
	p, a := setupDetail(t, src)

	onMain(t, a.app, func() {
		p.OnContext(uictx.SessionContext{Session: threeSessions[0]})
		p.OnContext(uictx.SQLContext{SQLID: "sql3"}) // no text: taken from stats
	})
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "SQL ID: sql3") }, "rendered sql3")
	onMain(t, a.app, func() {
		text := p.plainText()
		if strings.Contains(text, "Session:") {
			t.Errorf("session section should be gone for a bare SQLContext:\n%s", text)
		}
		if !strings.Contains(text, "update t set x = 1") {
			t.Errorf("SQL text should fall back to V$SQL text:\n%s", text)
		}
		if !strings.Contains(text, "No execution plan found") {
			t.Errorf("missing 'no plan' note:\n%s", text)
		}
	})
}

func TestSQLDetail_StaleResultIsDiscarded(t *testing.T) {
	src := newFakeSQL()
	p, a := setupDetail(t, src)

	// Simulate a slow fetch for sql1 that finishes after sql3 was selected.
	var staleGen uint64
	onMain(t, a.app, func() { staleGen = p.gen.Add(1) })
	onMain(t, a.app, func() { p.OnContext(uictx.SessionContext{Session: threeSessions[2]}) })
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "SQL ID: sql3") }, "rendered sql3")

	p.fetchAndRender(staleGen, &threeSessions[0], "sql1", "select 1 from dual")
	onMain(t, a.app, func() {
		if text := p.plainText(); !strings.Contains(text, "SQL ID: sql3") || strings.Contains(text, "sql1") {
			t.Errorf("stale sql1 result overwrote sql3:\n%s", text)
		}
	})
}

func TestSQLDetail_DBErrorsAreReported(t *testing.T) {
	src := newFakeSQL()
	src.planErr = errors.New("ORA-00942: table or view does not exist")
	src.statsErr = errors.New("ORA-01031: insufficient privileges")
	p, a := setupDetail(t, src)

	var mu sync.Mutex
	var reported []error
	p.SetStatusFn(func(err error) {
		mu.Lock()
		reported = append(reported, err)
		mu.Unlock()
	})

	onMain(t, a.app, func() { p.OnContext(uictx.SQLContext{SQLID: "sql1"}) })
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "SQL ID: sql1") }, "rendered")

	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 2 {
		t.Fatalf("reported %d errors, want 2: %v", len(reported), reported)
	}
	if !errors.Is(reported[0], src.planErr) || !errors.Is(reported[1], src.statsErr) {
		t.Errorf("reported = %v", reported)
	}
}

func TestSQLDetail_BracketsAreShownLiterally(t *testing.T) {
	src := newFakeSQL()
	src.plans["sqlx"] = []models.PlanRow{{Operation: "INDEX", Options: "RANGE SCAN", ObjectName: "IX[red]"}}
	p, a := setupDetail(t, src)

	s := models.Session{SID: 9, SQLID: "sqlx", SQLText: "select a[1] from t", Program: "app[yellow]"}
	onMain(t, a.app, func() { p.OnContext(uictx.SessionContext{Session: s}) })
	eventually(t, a.app, func() bool { return strings.Contains(p.plainText(), "Execution Plan") }, "rendered")
	onMain(t, a.app, func() {
		text := p.plainText()
		for _, want := range []string{"select a[1] from t", "app[yellow]", "INDEX RANGE SCAN IX[red]"} {
			if !strings.Contains(text, want) {
				t.Errorf("missing literal %q (swallowed as style tag?)\n%s", want, text)
			}
		}
	})
}
