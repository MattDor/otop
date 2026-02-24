package models

// Session represents a row from V$SESSION joined with V$SQL.
type Session struct {
	SID          int
	Serial       int
	Username     string
	Status       string
	SQLID        string
	SQLText      string
	Program      string
	Machine      string
	WaitEvent    string
	WaitSeconds  float64
	CPUTime      float64
	ElapsedTime  float64
	PhysicalReads int64
	LogicalReads  int64
}

// PlanRow represents a single step in an execution plan from V$SQL_PLAN.
type PlanRow struct {
	ID          int
	ParentID    int
	Depth       int
	Operation   string
	Options     string
	ObjectName  string
	Cardinality int64
	Bytes       int64
	Cost        int64
}

// SQLStats holds runtime statistics for a SQL statement from V$SQL.
type SQLStats struct {
	SQLID          string
	SQLText        string
	Executions     int64
	ElapsedTimeMicros int64
	CPUTimeMicros  int64
	BufferGets     int64
	DiskReads      int64
	Rows           int64
}
