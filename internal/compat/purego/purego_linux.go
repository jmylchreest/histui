//go:build linux

// Package purego bridges the small Linux API used by Oto to the same FFI
// runtime as GTK. Linking both forks introduces duplicate assembly symbols and
// two competing fake-cgo runtimes. Keep this surface deliberately small: a new
// Oto dependency on another API must fail compilation for explicit review.
package purego

import ffi "codeberg.org/puregotk/purego"

const (
	RTLD_LAZY   = ffi.RTLD_LAZY
	RTLD_NOW    = ffi.RTLD_NOW
	RTLD_LOCAL  = ffi.RTLD_LOCAL
	RTLD_GLOBAL = ffi.RTLD_GLOBAL
)

func Dlopen(path string, mode int) (uintptr, error)      { return ffi.Dlopen(path, mode) }
func Dlclose(handle uintptr) error                       { return ffi.Dlclose(handle) }
func Dlsym(handle uintptr, name string) (uintptr, error) { return ffi.Dlsym(handle, name) }
func RegisterLibFunc(target any, handle uintptr, name string) {
	ffi.RegisterLibFunc(target, handle, name)
}
