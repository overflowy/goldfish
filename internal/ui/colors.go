package ui

import (
	"fmt"
	"strconv"
)

// Palette
const (
	colorFocus     = "#ba4949"
	colorBreak     = "#2f5d50"
	colorLongBreak = "#2f5266"
	colorIdle      = "#d08d1e"

	colorTimeText  = "#f5f5fa"
	colorPhaseText = "#b8b8c8"
)

const borderDarken = 0.90
const trayLighten = 1.15

// scaleColor multiplies each RGB component of an "#rrggbb" colour by f, clamps to
// [0,255], and returns the new hex. f < 1 darkens, f > 1 lightens.
func scaleColor(hex string, f float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	component := func(s string) int {
		v, _ := strconv.ParseInt(s, 16, 0)
		return min(max(int(float64(v)*f), 0), 255)
	}
	return fmt.Sprintf("#%02x%02x%02x", component(hex[1:3]), component(hex[3:5]), component(hex[5:7]))
}

func darken(hex string, f float64) string  { return scaleColor(hex, f) }
func lighten(hex string, f float64) string { return scaleColor(hex, f) }
