package session

import "time"

// Phase is the kind of stretch currently running, or Idle for nothing running.
type Phase int

const (
	Idle Phase = iota
	Focus
	Break
	LongBreak
)

const blocksPerLongBreak = 4

type Durations struct {
	Focus     time.Duration
	Break     time.Duration
	LongBreak time.Duration
}

// Session is the live Cycle. Elapsed time is derived from a monotonic anchor
// rather than ticked, so it stays correct regardless of how often the UI polls.
type Session struct {
	durations Durations

	phase Phase

	// focusDone counts completed focus blocks toward the next Long break (0..4).
	// "Completed" means the user advanced out of focus via TakeBreak; an
	// Abandoned block does not increment it.
	focusDone int

	running bool
	paused  bool
	anchor  time.Time     // start of the current un-paused run segment
	base    time.Duration // elapsed accumulated before the current segment

	// autoStartBreaks gates focus→break, autoStartFocus gates break→focus. When
	// the relevant one is off, the phase stops at 0:00 and waits for the user.
	autoStartBreaks bool
	autoStartFocus  bool
	zeroChimed      bool // the time-up chime has fired for the current phase
}

func New(d Durations) *Session {
	return &Session{durations: d, phase: Idle, autoStartBreaks: true, autoStartFocus: true}
}

func (s *Session) SetDurations(d Durations) { s.durations = d }

func (s *Session) AutoStartBreaks() bool     { return s.autoStartBreaks }
func (s *Session) SetAutoStartBreaks(v bool) { s.autoStartBreaks = v }
func (s *Session) AutoStartFocus() bool      { return s.autoStartFocus }
func (s *Session) SetAutoStartFocus(v bool)  { s.autoStartFocus = v }

func (s *Session) Phase() Phase   { return s.phase }
func (s *Session) Running() bool  { return s.running }
func (s *Session) Paused() bool   { return s.paused }
func (s *Session) FocusDone() int { return s.focusDone }

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

func (s *Session) Elapsed() time.Duration {
	e := s.base
	if s.running && !s.paused {
		e += time.Since(s.anchor)
	}
	return e
}

// Remaining floors at zero. When Idle it reports the focus length so the overlay
// can preview the next block's duration.
func (s *Session) Remaining() time.Duration {
	if s.phase == Idle {
		return s.durations.Focus
	}
	if r := s.nominal() - s.Elapsed(); r > 0 {
		return r
	}
	return 0
}

// Tick advances the cycle when the running phase reaches its length, and reports
// whether the UI should chime. One hand-off per call (the next phase starts from
// zero), so a long sleep doesn't cascade through phases. A gated phase chimes
// once and waits.
func (s *Session) Tick() bool {
	if !s.running || s.paused || s.Elapsed() < s.nominal() {
		return false
	}
	switch s.phase {
	case Focus:
		if s.autoStartBreaks {
			s.TakeBreak()
			return true
		}
		return s.gatedChime()
	case Break, LongBreak:
		if s.autoStartFocus {
			s.StartNextFocus()
			return true
		}
		return s.gatedChime()
	default:
		return false
	}
}

func (s *Session) gatedChime() bool {
	if !s.zeroChimed {
		s.zeroChimed = true
		return true
	}
	return false
}

func (s *Session) StartFocus() {
	if s.phase != Idle {
		return
	}
	s.begin(Focus)
}

// TakeBreak ends the focus block counting it as completed and starts the
// appropriate rest, whether taken early or on the auto hand-off at zero.
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

// Abandon ends the focus block without counting it (focusDone untouched) and
// returns to Idle.
func (s *Session) Abandon() {
	if s.phase != Focus {
		return
	}
	s.toIdle()
}

// StartNextFocus ends the current break and begins the next focus block; after a
// Long break the cycle resets to block 1.
func (s *Session) StartNextFocus() {
	if s.phase != Break && s.phase != LongBreak {
		return
	}
	if s.phase == LongBreak {
		s.focusDone = 0
	}
	s.begin(Focus)
}

// Stop ends the whole cycle, discarding progress toward the Long break.
func (s *Session) Stop() {
	s.focusDone = 0
	s.toIdle()
}

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

func (s *Session) begin(p Phase) {
	s.phase = p
	s.base = 0
	s.anchor = time.Now()
	s.running = true
	s.paused = false
	s.zeroChimed = false
}

func (s *Session) toIdle() {
	s.phase = Idle
	s.base = 0
	s.running = false
	s.paused = false
}
