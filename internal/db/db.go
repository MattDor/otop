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

// GetActiveSessions returns all active sessions with their current SQL.
func (db *DB) GetActiveSessions() ([]models.Session, error) {
	// TODO: implement query against V$SESSION, V$SQL, V$SESSION_WAIT
	return nil, nil
}

// GetExecutionPlan returns the execution plan for the given SQL ID.
func (db *DB) GetExecutionPlan(sqlID string) ([]models.PlanRow, error) {
	// TODO: implement query against V$SQL_PLAN
	return nil, nil
}

// GetSQLStats returns runtime statistics for the given SQL ID.
func (db *DB) GetSQLStats(sqlID string) (*models.SQLStats, error) {
	// TODO: implement query against V$SQL
	return nil, nil
}
