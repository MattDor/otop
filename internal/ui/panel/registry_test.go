package panel

import "testing"

func TestRegistry(t *testing.T) {
	r := &Registry{}
	r.Register(Entry{TypeName: "A", Description: "first"})
	r.Register(Entry{TypeName: "B", Description: "second"})

	if e, ok := r.Get("B"); !ok || e.Description != "second" {
		t.Errorf("Get(B) = %+v, %v", e, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) reported found")
	}

	all := r.All()
	if len(all) != 2 || all[0].TypeName != "A" || all[1].TypeName != "B" {
		t.Fatalf("All() = %+v, want [A B] in registration order", all)
	}
	all[0].TypeName = "mutated"
	if e, _ := r.Get("A"); e.TypeName != "A" {
		t.Error("All() must return a copy")
	}
}
