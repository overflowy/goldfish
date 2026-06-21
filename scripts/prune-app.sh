#!/usr/bin/env bash
#
# Prune unwanted Qt plugins from a macOS .app bundle, then remove any
# framework/dylib that is left unreferenced (orphan garbage-collection).
#
# You only ever list PLUGINS to drop. macdeployqt copies whole frameworks in
# only because some plugin depends on them, so once the plugin is gone its
# frameworks become orphans and are swept automatically by walking the real
# Mach-O link graph (otool -L). This keeps the denylist small and correct:
# no need to hand-track transitive libs like libssl / libwebp / libjasper.
#
# Usage: prune-app.sh <Bundle.app> [plugin-relpath ...]
set -euo pipefail

BUNDLE="$1"
shift
C="$BUNDLE/Contents"
FW="$C/Frameworks"
PL="$C/PlugIns"

# 1) Delete the denylisted plugins.
for p in "$@"; do
	if [ -e "$PL/$p" ]; then
		rm -f "$PL/$p"
		echo "  drop plugin : $p"
	fi
done
# Remove now-empty plugin category directories.
find "$PL" -type d -empty -delete 2>/dev/null || true

# --- helpers --------------------------------------------------------------

# grep term that identifies references to a framework dir or a dylib.
macho_term() {
	local n
	n="$(basename "$1")"
	case "$n" in
	*.framework) echo "/${n}/" ;; # e.g. "/QtQuick.framework/"
	*) echo "/${n}" ;;            # e.g. "/libssl.3.dylib"
	esac
}

# The actual Mach-O file inside a framework dir, or the dylib itself.
macho_of() {
	local c="$1" n
	n="$(basename "$1")"
	if [[ "$c" == *.framework ]]; then echo "$c/Versions/A/${n%.framework}"; else echo "$c"; fi
}

# Frameworks/dylibs that are candidates for GC.
list_candidates() {
	for d in "$FW"/*.framework "$FW"/*.dylib; do [ -e "$d" ] && echo "$d"; done
}

# Every Mach-O that may reference a framework: the app binary, every
# surviving plugin, and every surviving framework. Plugins + binary are roots
# (never GC'd) because Qt loads plugins via dlopen, not via link records.
consumers() {
	echo "$C/MacOS/"*
	find "$PL" -name '*.dylib' 2>/dev/null || true
	for d in "$FW"/*.framework "$FW"/*.dylib; do [ -e "$d" ] && macho_of "$d"; done
}

# 2) Iteratively GC orphaned frameworks until the set stabilises (removing
#    QtQuick can orphan QtQml, which can orphan QtNetwork, ...).
changed=1
while [ "$changed" = 1 ]; do
	changed=0
	while IFS= read -r cand; do
		[ -e "$cand" ] || continue
		term="$(macho_term "$cand")"
		self="$(macho_of "$cand")"
		referenced=0
		while IFS= read -r f; do
			[ -f "$f" ] || continue
			[ "$f" = "$self" ] && continue
			if otool -L "$f" 2>/dev/null | grep -Fq "$term"; then
				referenced=1
				break
			fi
		done < <(consumers)
		if [ "$referenced" = 0 ]; then
			rm -rf "$cand"
			echo "  gc orphan   : $(basename "$cand")"
			changed=1
		fi
	done < <(list_candidates)
done

echo "  prune done"
