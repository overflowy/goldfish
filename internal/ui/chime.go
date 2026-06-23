package ui

import "os/exec"

// afplay + a built-in system sound keeps Goldfish free of QtMultimedia (not
// bundled) and of any shipped audio asset.
const chimeSound = "/System/Library/Sounds/Glass.aiff"

// playChime is fire-and-forget; a failure to play is silently ignored.
func playChime() {
	cmd := exec.Command("afplay", chimeSound)
	if err := cmd.Start(); err != nil {
		return
	}
	go cmd.Wait() // reap, don't care when
}
