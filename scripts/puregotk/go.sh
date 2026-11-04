#!/bin/sh
# Reproducible build with the two reviewed upstream fixes. No module-cache edits.
# Uses a private copy of the checksum-verified upstream Go module, cached by patch
# digest. Remove this wrapper when the fixes are available in a pinned release.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"
version=v0.0.0-20261003054014-d92056b0aa41
patches="$root/scripts/puregotk/patches"
key=$((cat "$patches"/*.patch; cat "$root/scripts/puregotk/go.sh") | sha256sum | cut -d' ' -f1)
cache="$root/.cache/puregotk/$key"
mkdir -p "$root/.cache/puregotk"
# Serialize initialization and modfile refresh across simultaneous builds.
exec 9>"$root/.cache/puregotk/prepare.lock"
flock 9
if [ ! -f "$cache/ready" ]; then
 module=$(go mod download -json "codeberg.org/puregotk/puregotk@$version" | sed -n 's/.*"Dir": "\(.*\)",/\1/p')
 [ -n "$module" ] || { echo 'Unable to locate verified puregotk module' >&2; exit 1; }
 staging=$(mktemp -d "$root/.cache/puregotk/staging.XXXXXX")
 trap 'rm -rf "$staging"' EXIT HUP INT TERM
 cp -R "$module" "$staging/upstream"
 chmod -R u+w "$staging/upstream"
 git -C "$staging/upstream" init -q
 for patch in "$patches"/*.patch; do
  git -C "$staging/upstream" apply --check "$patch"
  git -C "$staging/upstream" apply "$patch"
 done
 touch "$staging/ready"
 mv "$staging" "$cache"
 trap - EXIT HUP INT TERM
fi
# Use a per-invocation modfile so parallel commands never modify each other's sums.
moddir=$(mktemp -d "$root/.cache/puregotk/mod.XXXXXX")
trap 'rm -rf "$moddir"' EXIT HUP INT TERM
cp go.mod "$moddir/build.mod"
cp go.sum "$moddir/build.sum"
GOWORK=off go mod edit -modfile "$moddir/build.mod" -replace "codeberg.org/puregotk/puregotk=$cache/upstream"
flock -u 9
export CGO_ENABLED=${CGO_ENABLED:-0}
if [ "${1:-}" = "--tool" ]; then
 shift
 printf 'go 1.26.0\nuse "%s"\nreplace codeberg.org/puregotk/puregotk => "%s"\n' "$root" "$cache/upstream" > "$moddir/build.work"
 export GOWORK="$moddir/build.work"
 export GOFLAGS="${GOFLAGS:-} -tags=puregotk_layershell"
 "$@"
else
 export GOWORK=off
 export GOFLAGS="${GOFLAGS:-} -modfile=$moddir/build.mod -tags=puregotk_layershell"
 go "$@"
fi
