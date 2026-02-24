package context

import "github.com/mdoeren/otop/internal/models"

// Context is a closed sum type for workflow context values.
// The unexported method prevents external packages from implementing this interface.
type Context interface {
	contextType() string
}

// SessionContext carries the currently selected Oracle session.
type SessionContext struct {
	Session models.Session
}

func (SessionContext) contextType() string { return "SessionContext" }

// SQLContext carries the currently focused SQL ID and text.
type SQLContext struct {
	SQLID   string
	SQLText string
}

func (SQLContext) contextType() string { return "SQLContext" }
