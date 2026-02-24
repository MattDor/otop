# otop

A terminal-based Oracle database monitoring tool, inspired by htop.

## Features

- Live list of active Oracle sessions with SQL text and wait events
- Execution plan and runtime statistics for any selected SQL statement
- Extensible panel system: open, close, and resize panels freely
- Multiple workflow tabs for different monitoring contexts
- Command palette to add panels without leaving the keyboard

## Requirements

- Go 1.21+
- [Oracle Instant Client](https://www.oracle.com/database/technologies/instant-client.html) installed and on `LD_LIBRARY_PATH` (required at runtime by the `godror` driver)
- Access to an Oracle database with read permissions on `V$SESSION`, `V$SQL`, `V$SQL_PLAN`, `V$SESSTAT`, `V$SYSSTAT`, `V$SESSION_WAIT`

## Build

```sh
go build ./...
```

## Run

```sh
go run . -conn "user/password@host:port/service"
```

### Connection string format

```
user/password@host:port/service
```

Example:

```
scott/tiger@db.example.com:1521/ORCL
```

## Keybindings

| Key | Action |
|---|---|
| `Ctrl+P` | Open command palette |
| `Ctrl+W` | Close the focused panel |
| `Tab` | Move focus to the next panel |
| `Shift+Tab` | Move focus to the previous panel |
| `Ctrl+Right` | Switch to the next workflow tab |
| `Ctrl+Left` | Switch to the previous workflow tab |
| `Alt+Right` | Widen the focused panel |
| `Alt+Left` | Narrow the focused panel |
| `Alt+Down` | Taller focused panel |
| `Alt+Up` | Shorter focused panel |
| `Enter` (sessions list) | Select session → populate SQL Detail panel |
| `Esc` (palette) | Close command palette |

## Panels

| Panel | Description |
|---|---|
| **SessionList** | Table of active Oracle sessions. Selecting a row emits session and SQL context to other panels. Refreshes every 5 seconds. |
| **SQLDetail** | Shows execution plan steps and runtime statistics (executions, elapsed time, CPU, buffer gets, disk reads) for the selected SQL ID. |
| **QueryEditor** | Text editor pre-populated with the selected SQL statement. Planned for future query execution. |

## Architecture

```
internal/
├── db/             Oracle connection and query layer
├── models/         Shared data types (Session, PlanRow, SQLStats)
└── ui/
    ├── app.go                    Entry point for the TUI; wires all subsystems
    ├── context/
    │   ├── context.go            Closed-sum Context type (SessionContext, SQLContext)
    │   └── bus.go                Workflow-scoped pub/sub bus
    ├── panel/
    │   ├── panel.go              Panel interface + Emitter optional interface
    │   └── registry.go           Global panel registry (populated by init())
    ├── layout/
    │   ├── node.go               Binary layout tree (Split / Leaf nodes)
    │   └── builder.go            Converts node tree → tview.Flex tree
    ├── workflow/
    │   ├── workflow.go           Tab unit: owns bus, layout tree, refresh ticker
    │   └── manager.go            Tab bar + Pages switching
    ├── palette/
    │   └── palette.go            Command palette modal overlay
    └── panels/
        ├── sessions.go           SessionListPanel
        ├── sqldetail.go          SQLDetailPanel
        └── queryeditor.go        QueryEditorPanel (stub)
```

### Context bus

Panels communicate through a per-workflow pub/sub bus. When the user selects a session, `SessionListPanel` emits a `SessionContext` and a `SQLContext`. Any panel that declares those type names in `Subscriptions()` receives the value via `OnContext`. The bus is synchronous and runs on the tview main goroutine; panels must not block in `OnContext` — spawn a goroutine for any I/O and push UI updates back with `app.QueueUpdateDraw`.

### Layout tree

The panel layout is a binary tree of `layout.Node` values. Internal nodes are horizontal or vertical splits; leaf nodes hold a `tview.Primitive`. The tree is converted to a `tview.Flex` hierarchy on every layout change. Adding a panel splits the target leaf; removing one collapses any resulting single-child split.

### Adding a new panel

1. Create `internal/ui/panels/mypanel.go` implementing `panel.Panel`.
2. Register it in an `init()` function:

```go
func init() {
    panel.Global.Register(panel.Entry{
        TypeName:    "MyPanel",
        Description: "What it does",
        Factory:     newMyPanel,
    })
}
```

3. The panel automatically appears in the command palette (`Ctrl+P`). No other files need to change.

## Development

```sh
go build ./...      # build
go test ./...       # run tests
go vet ./...        # static analysis
```

### Oracle views used

| View | Purpose |
|---|---|
| `V$SESSION` | Active sessions |
| `V$SQL` | SQL text and runtime statistics |
| `V$SQL_PLAN` | Execution plan steps |
| `V$SESSION_WAIT` | Current wait event per session |
| `V$SESSTAT` / `V$SYSSTAT` | Session and system statistics |
