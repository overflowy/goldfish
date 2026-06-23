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

func TestTickAutoAdvancesAtZero(t *testing.T) {
	s := New(Durations{Focus: 10 * time.Millisecond, Break: time.Minute, LongBreak: time.Minute})
	s.StartFocus()

	// Before the duration elapses, Tick is a no-op.
	if s.Tick() {
		t.Fatal("Tick advanced before the duration elapsed")
	}
	if s.Phase() != Focus {
		t.Fatalf("phase changed early: %v", s.Phase())
	}

	time.Sleep(25 * time.Millisecond)

	// Once elapsed, Tick hands off to the break and counts the block.
	if !s.Tick() {
		t.Fatal("Tick did not advance after the duration elapsed")
	}
	if s.Phase() != Break {
		t.Fatalf("want Break after auto-advance, got %v", s.Phase())
	}
	if s.FocusDone() != 1 {
		t.Fatalf("auto-advanced focus block did not count: FocusDone = %d", s.FocusDone())
	}
	// Remaining never goes negative now; it floors at zero.
	if s.Remaining() < 0 {
		t.Fatalf("Remaining went negative: %v", s.Remaining())
	}
}

func TestTickGatesFocusWhenAutoStartOff(t *testing.T) {
	s := New(Durations{Focus: 10 * time.Millisecond, Break: time.Minute, LongBreak: time.Minute})
	s.SetAutoStartBreaks(false)
	s.StartFocus()
	time.Sleep(25 * time.Millisecond)

	// First tick past zero chimes once but stays in Focus (waiting for the user).
	if !s.Tick() {
		t.Fatal("expected a one-shot chime when focus time is up")
	}
	if s.Phase() != Focus {
		t.Fatalf("focus auto-advanced with auto-start off: %v", s.Phase())
	}
	// Subsequent ticks neither chime again nor advance.
	if s.Tick() {
		t.Fatal("chimed/advanced more than once while gated")
	}
	if s.Phase() != Focus {
		t.Fatalf("focus advanced on a later tick: %v", s.Phase())
	}
	// The user takes the break manually.
	s.TakeBreak()
	if s.Phase() != Break {
		t.Fatalf("manual TakeBreak failed: %v", s.Phase())
	}
}

func TestTickGatesBreakWhenAutoStartFocusOff(t *testing.T) {
	s := New(Durations{Focus: time.Minute, Break: 10 * time.Millisecond, LongBreak: time.Minute})
	s.SetAutoStartFocus(false)
	s.StartFocus()
	s.TakeBreak() // now in Break
	time.Sleep(25 * time.Millisecond)

	if !s.Tick() {
		t.Fatal("expected a one-shot chime when break time is up")
	}
	if s.Phase() != Break {
		t.Fatalf("break auto-advanced with auto-start-focus off: %v", s.Phase())
	}
	if s.Tick() {
		t.Fatal("chimed/advanced more than once while gated")
	}
	// The user starts the next focus manually.
	s.StartNextFocus()
	if s.Phase() != Focus {
		t.Fatalf("manual StartNextFocus failed: %v", s.Phase())
	}
}

func TestTickDoesNotAdvanceWhilePaused(t *testing.T) {
	s := New(Durations{Focus: 10 * time.Millisecond, Break: time.Minute, LongBreak: time.Minute})
	s.StartFocus()
	s.Pause()
	time.Sleep(25 * time.Millisecond)
	if s.Tick() {
		t.Fatal("Tick advanced while paused")
	}
	if s.Phase() != Focus {
		t.Fatalf("paused phase changed: %v", s.Phase())
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
