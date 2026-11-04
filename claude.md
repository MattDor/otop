# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Goal

Develop a terminal-based Oracle database monitoring tool, similar to htop for Linux.

**Constraints:**
- Written in Go
- Runs in a Linux terminal

**Success criteria:**
- Connect to an Oracle database
- Display a list of currently running SQL statements
- Show details for a single SQL statement: execution plan, runtime statistics, resource consumption

## Development Commands

```sh
go build ./...           # Build the project
go run . -conn <str>     # Run (connection string: user/password@host:port/service)
go test ./...            # Run all tests
go test ./internal/db/   # Run tests for a specific package
go vet ./...             # Static analysis
```

## Architecture

- **`main.go`** — Entry point, parses `-conn` flag, wires up DB connection and TUI
- **`internal/models/`** — Shared data types (`Session`, `PlanRow`, `SQLStats`)
- **`internal/db/`** — Oracle connection and query layer (`Connect`, `GetActiveSessions`, `GetExecutionPlan`, `GetSQLStats`)
- **`internal/ui/`** — tview application (`App`), session list table + detail panel in a two-row `Grid`

### Key Oracle Views

- `V$SESSION` / `V$SQL` — active sessions and SQL text
- `V$SQL_PLAN` — execution plans
- `V$SESSTAT` / `V$SYSSTAT` — runtime statistics
- `V$SESSION_WAIT` — resource consumption / wait events

### Dependencies

- **Oracle driver:** `github.com/godror/godror` (CGO, requires Oracle Instant Client at runtime)
- **TUI framework:** `github.com/rivo/tview` (widget-based, `Table` + `Grid` primitives)
