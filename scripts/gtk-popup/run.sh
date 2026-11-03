#!/bin/sh
set -eu
builddir=$(mktemp -d /tmp/histui-pointer.XXXXXX)
trap 'rm -rf "$builddir"' EXIT
src=$(dirname "$0")
wayland-scanner client-header "$src/wlr-virtual-pointer-unstable-v1.xml" "$builddir/virtual-pointer.h"
wayland-scanner private-code "$src/wlr-virtual-pointer-unstable-v1.xml" "$builddir/virtual-pointer.c"
cc -Wall -Wextra -Werror -O2 -I"$builddir" "$src/pointer.c" "$builddir/virtual-pointer.c" -o "$builddir/pointer" $(pkg-config --cflags --libs wayland-client)
export HISTUI_TEST_POINTER="$builddir/pointer"
# A new bus prevents connecting to or replacing the user's notification daemon.
dbus-run-session -- python3 "$src/capture.py" "$@"
