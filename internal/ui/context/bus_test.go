package context

import (
	"testing"

	"github.com/mdoeren/otop/internal/models"
)

func TestBus_EmitDispatchesByType(t *testing.T) {
	b := NewBus()
	var sessions, sqls int
	b.Subscribe("SessionContext", func(Context) { sessions++ })
	b.Subscribe("SQLContext", func(Context) { sqls++ })

	b.Emit(SessionContext{Session: models.Session{SID: 1}})
	b.Emit(SQLContext{SQLID: "x"})
	b.Emit(SQLContext{SQLID: "y"})

	if sessions != 1 || sqls != 2 {
		t.Errorf("sessions=%d sqls=%d, want 1 and 2", sessions, sqls)
	}
}

func TestBus_UnsubscribeRemovesOnlyThatHandler(t *testing.T) {
	b := NewBus()
	var a, c int
	unsubA := b.Subscribe("SQLContext", func(Context) { a++ })
	b.Subscribe("SQLContext", func(Context) { c++ })

	unsubA()
	unsubA() // idempotent
	b.Emit(SQLContext{})

	if a != 0 || c != 1 {
		t.Errorf("a=%d c=%d, want 0 and 1", a, c)
	}
}

func TestBus_HandlerMayUnsubscribeDuringEmit(t *testing.T) {
	b := NewBus()
	calls := 0
	var unsub func()
	unsub = b.Subscribe("SQLContext", func(Context) {
		calls++
		unsub() // must not deadlock
	})
	b.Emit(SQLContext{})
	b.Emit(SQLContext{})
	if calls != 1 {
		t.Errorf("calls=%d, want 1", calls)
	}
}

func TestSubscribeGeneric(t *testing.T) {
	b := NewBus()
	var got []string
	Subscribe(b, func(c SQLContext) { got = append(got, c.SQLID) })

	b.Emit(SQLContext{SQLID: "abc"})
	b.Emit(SessionContext{}) // different type: ignored

	if len(got) != 1 || got[0] != "abc" {
		t.Errorf("got %v, want [abc]", got)
	}
}
