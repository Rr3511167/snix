//go:build !windows

// Stub so `go test ./...` and cross-compile on non-Windows hosts succeeds.
// Real code lives in backend.go and windivert_api.go, both guarded by
// `//go:build windows`.
package windows
