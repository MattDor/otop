package db

import (
	"database/sql"
	"fmt"

	"github.com/mdoeren/otop/internal/models"
	_ "github.com/godror/godror"
)

// DB wraps a sql.DB connection to Oracle.
type DB struct {
	conn *sql.DB
}

// Connect opens a connection to Oracle using a godror connection string.
// connStr format: user/password@host:port/service
func Connect(connStr string) (*DB, error) {
	conn, err := sql.Open("godror", connStr)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetActiveSessions returns all user sessions joined with their current SQL
// text and wait event. Active sessions sort first.
func (db *DB) GetActiveSessions() ([]models.Session, error) {
	const query = `
SELECT
    s.SID,
    s.SERIAL#,
    NVL(s.USERNAME, '(background)')  AS USERNAME,
    s.STATUS,
    NVL(s.SQL_ID, '')                AS SQL_ID,
    NVL(q.SQL_TEXT, '')              AS SQL_TEXT,
    NVL(s.PROGRAM, '')               AS PROGRAM,
    NVL(s.MACHINE, '')               AS MACHINE,
    NVL(w.EVENT, '')                 AS WAIT_EVENT,
    NVL(w.SECONDS_IN_WAIT, 0)        AS WAIT_SECONDS,
    NVL(q.CPU_TIME,     0) / 1e6    AS CPU_TIME,
    NVL(q.ELAPSED_TIME, 0) / 1e6    AS ELAPSED_TIME,
    NVL(q.DISK_READS,   0)           AS PHYSICAL_READS,
    NVL(q.BUFFER_GETS,  0)           AS LOGICAL_READS
FROM V$SESSION s
LEFT JOIN V$SQL q
       ON s.SQL_ID          = q.SQL_ID
      AND s.SQL_CHILD_NUMBER = q.CHILD_NUMBER
LEFT JOIN V$SESSION_WAIT w
       ON s.SID = w.SID
WHERE s.TYPE = 'USER'
ORDER BY
    CASE s.STATUS WHEN 'ACTIVE' THEN 0 ELSE 1 END,
    s.SID`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("GetActiveSessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(
			&s.SID, &s.Serial, &s.Username, &s.Status,
			&s.SQLID, &s.SQLText, &s.Program, &s.Machine,
			&s.WaitEvent, &s.WaitSeconds,
			&s.CPUTime, &s.ElapsedTime,
			&s.PhysicalReads, &s.LogicalReads,
		); err != nil {
			return nil, fmt.Errorf("GetActiveSessions scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetExecutionPlan returns the execution plan rows for the given SQL ID,
// using the lowest child cursor number to get a consistent plan.
func (db *DB) GetExecutionPlan(sqlID string) ([]models.PlanRow, error) {
	const query = `
SELECT
    ID,
    NVL(PARENT_ID,   0)  AS PARENT_ID,
    DEPTH,
    OPERATION,
    NVL(OPTIONS,     '') AS OPTIONS,
    NVL(OBJECT_NAME, '') AS OBJECT_NAME,
    NVL(CARDINALITY, 0)  AS CARDINALITY,
    NVL(BYTES,       0)  AS BYTES,
    NVL(COST,        0)  AS COST
FROM V$SQL_PLAN
WHERE SQL_ID = :sqlid
  AND CHILD_NUMBER = (
      SELECT MIN(CHILD_NUMBER)
      FROM   V$SQL_PLAN
      WHERE  SQL_ID = :sqlid
  )
ORDER BY ID`

	rows, err := db.conn.Query(query, sql.Named("sqlid", sqlID))
	if err != nil {
		return nil, fmt.Errorf("GetExecutionPlan: %w", err)
	}
	defer rows.Close()

	var plan []models.PlanRow
	for rows.Next() {
		var r models.PlanRow
		if err := rows.Scan(
			&r.ID, &r.ParentID, &r.Depth,
			&r.Operation, &r.Options, &r.ObjectName,
			&r.Cardinality, &r.Bytes, &r.Cost,
		); err != nil {
			return nil, fmt.Errorf("GetExecutionPlan scan: %w", err)
		}
		plan = append(plan, r)
	}
	return plan, rows.Err()
}

// GetSQLStats returns aggregated runtime statistics for the given SQL ID,
// summing across all child cursors.
func (db *DB) GetSQLStats(sqlID string) (*models.SQLStats, error) {
	const query = `
SELECT
    SQL_ID,
    MIN(SQL_TEXT)          AS SQL_TEXT,
    SUM(EXECUTIONS)        AS EXECUTIONS,
    SUM(ELAPSED_TIME)      AS ELAPSED_TIME,
    SUM(CPU_TIME)          AS CPU_TIME,
    SUM(BUFFER_GETS)       AS BUFFER_GETS,
    SUM(DISK_READS)        AS DISK_READS,
    SUM(ROWS_PROCESSED)    AS ROWS_PROCESSED
FROM V$SQL
WHERE SQL_ID = :sqlid
GROUP BY SQL_ID`

	var s models.SQLStats
	err := db.conn.QueryRow(query, sql.Named("sqlid", sqlID)).Scan(
		&s.SQLID, &s.SQLText,
		&s.Executions, &s.ElapsedTimeMicros, &s.CPUTimeMicros,
		&s.BufferGets, &s.DiskReads, &s.Rows,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetSQLStats: %w", err)
	}
	return &s, nil
}
