package tokenexpiry

import (
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.Local)

	if at, ok, _ := Resolve("", now); !ok || !at.Equal(now.Add(24*time.Hour)) {
		t.Errorf("empty -> %v ok=%v, want +24h", at, ok)
	}
	if _, ok, msg := Resolve("2026-09", now); ok || msg == "" {
		t.Errorf("bad format should fail with a message, got ok=%v", ok)
	}
	if _, ok, _ := Resolve("20260230", now); ok {
		t.Error("Feb 30 does not exist")
	}
	if _, ok, _ := Resolve("20260901", now); ok {
		t.Error("a past date must be rejected")
	}
	if _, ok, _ := Resolve("20270101", now); ok {
		t.Error("more than 90 days away must be rejected")
	}
	at, ok, _ := Resolve("20260915", now)
	if !ok || at.Hour() != 23 || at.Minute() != 59 {
		t.Errorf("valid date -> %v ok=%v, want end-of-day", at, ok)
	}
}
