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
		s.StartFocus() // back into focus
	}
	s.TakeBreak() // completes the 4th focus block
	if s.Phase() != LongBreak {
		t.Fatalf("after 4th block: want LongBreak, got %v", s.Phase())
	}
	// Leaving the Long break resets the cycle.
	s.StartFocus()
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
	s.StartFocus()
	if s.Phase() != Focus {
		t.Fatalf("manual StartFocus failed: %v", s.Phase())
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

// atLongBreak drives the session through four completed focus blocks so it lands
// on the Long break.
func atLongBreak(s *Session) {
	for range 3 {
		s.StartFocus()
		s.TakeBreak()
		s.StartFocus()
	}
	s.TakeBreak() // 4th block completes -> Long break
}

// TestCapabilityMatrix pins which intents are valid in every reachable state.
// This is the logic the tray menu used to re-derive untested.
func TestCapabilityMatrix(t *testing.T) {
	cases := []struct {
		name                                                     string
		setup                                                    func(*Session)
		canStart, canBreak, canAbandon, canReset, canPauseResume bool
	}{
		{"idle", func(s *Session) {}, true, false, false, false, false},
		{"focus running", func(s *Session) { s.StartFocus() }, false, true, true, true, true},
		{"focus paused", func(s *Session) { s.StartFocus(); s.Pause() }, false, true, true, true, true},
		{"break running", func(s *Session) { s.StartFocus(); s.TakeBreak() }, true, false, false, true, true},
		{"break paused", func(s *Session) { s.StartFocus(); s.TakeBreak(); s.Pause() }, true, false, false, true, true},
		{"long break", atLongBreak, true, false, false, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestSession()
			c.setup(s)
			if got := s.CanStartFocus(); got != c.canStart {
				t.Errorf("CanStartFocus = %v, want %v", got, c.canStart)
			}
			if got := s.CanTakeBreak(); got != c.canBreak {
				t.Errorf("CanTakeBreak = %v, want %v", got, c.canBreak)
			}
			if got := s.CanAbandon(); got != c.canAbandon {
				t.Errorf("CanAbandon = %v, want %v", got, c.canAbandon)
			}
			if got := s.CanReset(); got != c.canReset {
				t.Errorf("CanReset = %v, want %v", got, c.canReset)
			}
			if got := s.CanPauseResume(); got != c.canPauseResume {
				t.Errorf("CanPauseResume = %v, want %v", got, c.canPauseResume)
			}
		})
	}
}

func TestToggleRun(t *testing.T) {
	t.Run("starts focus from idle", func(t *testing.T) {
		s := newTestSession()
		s.ToggleRun()
		if s.Phase() != Focus || !s.Running() || s.Paused() {
			t.Fatalf("after toggle from idle: phase=%v running=%v paused=%v", s.Phase(), s.Running(), s.Paused())
		}
	})
	t.Run("pauses a running phase", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		s.ToggleRun()
		if !s.Paused() || s.Phase() != Focus {
			t.Fatalf("after toggle while running: phase=%v paused=%v", s.Phase(), s.Paused())
		}
	})
	t.Run("resumes when paused", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		s.Pause()
		s.ToggleRun()
		if s.Paused() {
			t.Fatal("toggle did not resume a paused phase")
		}
	})
	t.Run("pauses during a break", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		s.TakeBreak()
		s.ToggleRun()
		if !s.Paused() || s.Phase() != Break {
			t.Fatalf("toggle during break: phase=%v paused=%v", s.Phase(), s.Paused())
		}
	})
}

func TestStartFocusUnified(t *testing.T) {
	t.Run("begins from idle", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		if s.Phase() != Focus {
			t.Fatalf("want Focus from idle, got %v", s.Phase())
		}
	})
	t.Run("advances out of a break without losing progress", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		s.TakeBreak() // FocusDone = 1, in Break
		s.StartFocus()
		if s.Phase() != Focus || s.FocusDone() != 1 {
			t.Fatalf("after break->focus: phase=%v focusDone=%d", s.Phase(), s.FocusDone())
		}
	})
	t.Run("resets the cycle leaving a long break", func(t *testing.T) {
		s := newTestSession()
		atLongBreak(s)
		if s.Phase() != LongBreak || s.FocusDone() != 4 {
			t.Fatalf("setup: phase=%v focusDone=%d", s.Phase(), s.FocusDone())
		}
		s.StartFocus()
		if s.Phase() != Focus || s.FocusDone() != 0 {
			t.Fatalf("after long break->focus: phase=%v focusDone=%d", s.Phase(), s.FocusDone())
		}
	})
	t.Run("is a no-op while already focusing", func(t *testing.T) {
		s := newTestSession()
		s.StartFocus()
		s.StartFocus()
		if s.Phase() != Focus {
			t.Fatalf("second StartFocus changed phase to %v", s.Phase())
		}
	})
}
