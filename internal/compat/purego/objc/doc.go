// Package objc is a module-graph placeholder for Oto's Darwin-only imports.
// histui's shared-FFI bridge supports Linux only; it deliberately supplies no
// Objective-C implementation. This lets go mod tidy inspect all platform imports
// without downloading a second FFI runtime. Non-Linux builds are unsupported.
package objc
