//go:build !amd64

package simdscan

// simdAvailable is false on non-amd64 architectures.
var simdAvailable = false

// classify32 is a no-op on non-amd64 — never called because simdAvailable is false.
func classify32(data *byte) (quotes, backslashes, structural uint32) {
	return 0, 0, 0
}
