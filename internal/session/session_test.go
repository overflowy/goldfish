package session

import (
	"testing"
	"time"
)

func newTestSession() *Session {
	return New(Durations{
		Focus:     25 * time.Minute,
		Break:     5 * time.Minute,
		LongBreak: 15 * time.Minute,
	})
}

func TestStartsIdle(t *testing.T) {
	s := newTestSession()
	if s.Phase() != Idle {
		t.Fatalf("want Idle, got %v", s.Phase())
	}
	// Idle previews the focus duration so the Start anchor can show it.
	if got := s.Remaining(); got != 25*time.Minute {
		t.Fatalf("idle Remaining = %v, want 25m", got)
	}
}

func TestFullCycleEarnsLongBreak(t *testing.T) {
	s := newTestSession()
	// Four focus blocks, each completed via TakeBreak. The first three give a
	// short Break; the fourth gives a Long break.
	for i := 1; i <= 3; i++ {
		s.StartFocus()
		s.TakeBreak()
		if s.Phase() != Break {
			t.Fatalf("block %d: want Break, got %v", i, s.Phase())
		}
		if s.FocusDone() != i {
			t.Fatalf("block %d: FocusDone = %d", i, s.FocusDone())
		}
		s.StartNextFocus() // back into focus
	}
	s.TakeBreak() // completes the 4th focus block
	if s.Phase() != LongBreak {
		t.Fatalf("after 4th block: want LongBreak, got %v", s.Phase())
	}
	// Leaving the Long break resets the cycle.
	s.StartNextFocus()
	if s.Phase() != Focus || s.FocusDone() != 0 {
		t.Fatalf("after long break: phase=%v focusDone=%d", s.Phase(), s.FocusDone())
	}
}

func TestAbandonDoesNotCount(t *testing.T) {
	s := newTestSession()
	s.StartFocus()
	s.Abandon()
	if s.Phase() != Idle {
		t.Fatalf("after abandon: want Idle, got %v", s.Phase())
	}
	if s.FocusDone() != 0 {
		t.Fatalf("abandon counted toward long break: FocusDone = %d", s.FocusDone())
	}
}

func TestOvertimeIsSignedAndNeverAutoAdvances(t *testing.T) {
	s := New(Durations{Focus: 10 * time.Millisecond, Break: time.Minute, LongBreak: time.Minute})
	s.StartFocus()
	time.Sleep(25 * time.Millisecond)
	if !s.Overtime() {
		t.Fatal("expected Overtime after nominal elapsed")
	}
	if s.Remaining() >= 0 {
		t.Fatalf("overtime Remaining should be negative, got %v", s.Remaining())
	}
	// The defining rule: time passing never changes the phase on its own.
	if s.Phase() != Focus {
		t.Fatalf("phase auto-advanced on its own: %v", s.Phase())
	}
}

func TestPauseFreezesElapsed(t *testing.T) {
	s := newTestSession()
	s.StartFocus()
	time.Sleep(15 * time.Millisecond)
	s.Pause()
	frozen := s.Elapsed()
	time.Sleep(20 * time.Millisecond)
	if drift := s.Elapsed() - frozen; drift > time.Millisecond {
		t.Fatalf("elapsed advanced while paused by %v", drift)
	}
	s.Resume()
	time.Sleep(15 * time.Millisecond)
	if s.Elapsed() <= frozen {
		t.Fatal("elapsed did not advance after resume")
	}
}

func TestStopResetsCycle(t *testing.T) {
	s := newTestSession()
	s.StartFocus()
	s.TakeBreak() // FocusDone = 1, in Break
	s.Stop()
	if s.Phase() != Idle || s.FocusDone() != 0 {
		t.Fatalf("after stop: phase=%v focusDone=%d", s.Phase(), s.FocusDone())
	}
}
