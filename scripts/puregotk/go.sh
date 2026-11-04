#!/bin/sh
# GTK must load layer-shell before generated GTK/GDK initialization.
# Dependency fixes are checksum-pinned in go.mod; no local patching is needed.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"
export CGO_ENABLED=${CGO_ENABLED:-0}
export GOFLAGS="${GOFLAGS:-} -tags=puregotk_layershell"
if [ "${1:-}" = "--tool" ]; then
 shift
 exec "$@"
else
 exec go "$@"
fi
