//go:build !linux

// Stub file so the linux package compiles (empty) on non-Linux hosts,
// allowing `go test ./...` to pass everywhere. Real code lives in nfqueue.go
// guarded by //go:build linux.
package linux
