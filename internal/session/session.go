// Package session is the pure-Go core of Goldfish: the Cycle state machine. It
// holds no Qt and knows nothing about rendering — the overlay polls it each tick
// and draws whatever it reports. The defining rule (see docs/adr/0001) is that
// phases never advance themselves: a phase runs past zero into Overtime forever
// and only an explicit method call ends it.
package session

import "time"

// Phase is which kind of stretch is currently running, or Idle for "nothing
// running" (fresh launch, or after the cycle was stopped or a block abandoned).
type Phase int

const (
	Idle Phase = iota
	Focus
	Break
	LongBreak
)

// blocksPerLongBreak is the baked-in "4" of classic Pomodoro: a Long break is
// earned after this many completed focus blocks. Deliberately not configurable.
const blocksPerLongBreak = 4

// Durations are the three tunable phase lengths, in real time.
type Durations struct {
	Focus     time.Duration
	Break     time.Duration
	LongBreak time.Duration
}

// Session is the live Cycle. Elapsed time is derived from a monotonic anchor
// rather than ticked, so it stays correct regardless of how often the UI polls.
type Session struct {
	durations Durations

	phase     Phase
	intention string

	// focusDone counts completed focus blocks toward the next Long break (0..4).
	// "Completed" means the user advanced out of focus via TakeBreak; an
	// Abandoned block does not increment it.
	focusDone int

	running bool          // a phase is active (Focus/Break/LongBreak)
	paused  bool          // running but clock frozen
	anchor  time.Time     // start of the current un-paused run segment
	base    time.Duration // elapsed accumulated before the current segment
}

// New returns an Idle session with the given durations.
func New(d Durations) *Session {
	return &Session{durations: d, phase: Idle}
}

// SetDurations updates the phase lengths; takes effect on the next phase start.
func (s *Session) SetDurations(d Durations) { s.durations = d }

// --- queries the overlay renders from -------------------------------------

func (s *Session) Phase() Phase          { return s.phase }
func (s *Session) Intention() string     { return s.intention }
func (s *Session) SetIntention(t string) { s.intention = t }
func (s *Session) Running() bool         { return s.running }
func (s *Session) Paused() bool          { return s.paused }

// FocusDone is how many focus blocks are complete in the current cycle (0..4),
// for rendering progress toward the Long break.
func (s *Session) FocusDone() int { return s.focusDone }

// nominal is the configured length of the current phase (0 when Idle).
func (s *Session) nominal() time.Duration {
	switch s.phase {
	case Focus:
		return s.durations.Focus
	case Break:
		return s.durations.Break
	case LongBreak:
		return s.durations.LongBreak
	default:
		return 0
	}
}

// Elapsed is how long the current phase has been running (frozen while paused).
func (s *Session) Elapsed() time.Duration {
	e := s.base
	if s.running && !s.paused {
		e += time.Since(s.anchor)
	}
	return e
}

// Remaining is signed time left in the phase: negative once in Overtime. When
// Idle it reports the configured focus length, so the overlay can preview the
// next block's duration on its "Start focus" anchor.
func (s *Session) Remaining() time.Duration {
	if s.phase == Idle {
		return s.durations.Focus
	}
	return s.nominal() - s.Elapsed()
}

// Overtime reports whether the current phase has run past its nominal length.
// Stays true while paused, so the overtime visual state is stable.
func (s *Session) Overtime() bool {
	return s.phase != Idle && s.Remaining() < 0
}

// --- transitions (the only things that change phase) ----------------------

// StartFocus begins a focus block from Idle. No-op if a phase is already running.
func (s *Session) StartFocus() {
	if s.phase != Idle {
		return
	}
	s.begin(Focus)
}

// TakeBreak ends the current focus block counting it as completed, and starts
// the appropriate rest. This is the normal forward path out of focus, whether
// taken in overtime or early ("skip to break") — either way the block counts.
func (s *Session) TakeBreak() {
	if s.phase != Focus {
		return
	}
	s.focusDone++
	if s.focusDone >= blocksPerLongBreak {
		s.begin(LongBreak)
	} else {
		s.begin(Break)
	}
}

// Abandon ends the current focus block without counting it and returns to Idle.
// The "I bailed" exit: focusDone is left untouched, so the voided block simply
// did not happen.
func (s *Session) Abandon() {
	if s.phase != Focus {
		return
	}
	s.toIdle()
}

// StartNextFocus ends the current break and begins the next focus block. After a
// Long break the cycle resets, so the next focus is block 1 of a fresh four.
func (s *Session) StartNextFocus() {
	if s.phase != Break && s.phase != LongBreak {
		return
	}
	if s.phase == LongBreak {
		s.focusDone = 0
	}
	s.begin(Focus)
}

// Stop ends the whole cycle and returns to Idle, discarding progress toward the
// Long break. Available from any running phase.
func (s *Session) Stop() {
	s.focusDone = 0
	s.toIdle()
}

// Pause freezes the running clock. Resume continues the same phase — a paused
// block is still that block. No-op if not running or already paused/resumed.
func (s *Session) Pause() {
	if !s.running || s.paused {
		return
	}
	s.base += time.Since(s.anchor)
	s.paused = true
}

func (s *Session) Resume() {
	if !s.running || !s.paused {
		return
	}
	s.anchor = time.Now()
	s.paused = false
}

// --- internal helpers ------------------------------------------------------

func (s *Session) begin(p Phase) {
	s.phase = p
	s.base = 0
	s.anchor = time.Now()
	s.running = true
	s.paused = false
}

func (s *Session) toIdle() {
	s.phase = Idle
	s.base = 0
	s.running = false
	s.paused = false
}
