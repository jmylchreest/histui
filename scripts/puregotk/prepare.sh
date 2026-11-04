#!/bin/sh
# Prepare a disposable patched upstream checkout and temporary root-module file.
set -eu
upstream=$(realpath "${1:?usage: prepare.sh /disposable/puregotk /temporary/output}")
out=$(realpath -m "${2:?temporary output directory required}")
script=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(realpath "$script/../..")
expected=d92056b0aa411de74046dd2748e06e23d65d7a09
[ "$(git -C "$upstream" rev-parse HEAD)" = "$expected" ] || { echo "Unexpected upstream revision" >&2; exit 1; }
for patch in "$script"/patches/*.patch; do
 if git -C "$upstream" apply --reverse --check "$patch" 2>/dev/null; then continue; fi
 git -C "$upstream" apply --check "$patch"
 git -C "$upstream" apply "$patch"
done
mkdir -p "$out"
cp "$root/go.mod" "$out/histui.mod"
cp "$root/go.sum" "$out/histui.sum"
cd "$root"
GOWORK=off go mod edit -modfile "$out/histui.mod" -replace "codeberg.org/puregotk/puregotk=$upstream"
printf '%s\n' "$out/histui.mod"
