package store

import "testing"

// Values captured from Node's Helpers/ClientDues.computeClientDues so the two can't drift.
func TestComputeClientDues(t *testing.T) {
	invoices := []ClientAmount{
		{"a", 1000}, {"a", 250.005}, {"b", 500}, {"c", 800}, {"", 999}, // "" is a detached client, aggregates nothing
	}
	received := []ClientAmount{{"a", 400}, {"b", 500}}
	batch := []ClientAmount{{"c", 300}, {"a", 100}}

	got := ComputeClientDues(invoices, received, batch)

	want := []ClientDue{
		{ClientID: "a", Billed: 1250.01, Received: 500, Due: 750.01},
		{ClientID: "c", Billed: 800, Received: 300, Due: 500},
		{ClientID: "b", Billed: 500, Received: 500, Due: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d rows, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestComputeClientDues_Empty(t *testing.T) {
	if got := ComputeClientDues(nil, nil, nil); len(got) != 0 {
		t.Errorf("empty inputs should yield no rows, got %+v", got)
	}
}
