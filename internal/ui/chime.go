package ui

import "os/exec"

// chimeSound is a built-in macOS system sound. Using afplay keeps Goldfish free
// of QtMultimedia (not bundled) and of any shipped audio asset.
const chimeSound = "/System/Library/Sounds/Glass.aiff"

// playChime sounds the single zero-crossing chime. It is fire-and-forget: per
// docs/adr/0001 the crossing chimes exactly once and never nags, so a failure to
// play (afplay missing, file absent) is silently ignored — the visual overtime
// flip is the primary, reliable signal.
func playChime() {
	cmd := exec.Command("afplay", chimeSound)
	if err := cmd.Start(); err != nil {
		return
	}
	// Reap the process so it doesn't linger as a zombie; we don't care when.
	go cmd.Wait()
}
