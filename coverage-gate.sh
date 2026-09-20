#!/usr/bin/env bash
# Fail when total statement coverage drops below the floor. The floor is a
# ratchet, not a target: raise it when coverage rises, never lower it to make a
# build pass.
set -euo pipefail
cd "$(dirname "$0")"

FLOOR="${COVERAGE_FLOOR:-65}"

profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT
CGO_ENABLED=0 go test -coverpkg=./... -coverprofile="$profile" ./... >/dev/null

total="$(go tool cover -func="$profile" | awk '/^total:/ {print $3}' | tr -d '%')"
printf 'total coverage: %s%% (floor %s%%)\n' "$total" "$FLOOR"

awk -v total="$total" -v floor="$FLOOR" 'BEGIN { exit !(total + 0 >= floor + 0) }' || {
	echo "coverage ${total}% is below the ${FLOOR}% floor" >&2
	exit 1
}
