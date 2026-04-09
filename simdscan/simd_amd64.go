//go:build amd64

package simdscan

// simdAvailable is true if the CPU supports AVX2.
var simdAvailable = hasAVX2()

// classify32 classifies 32 bytes starting at *data using AVX2 SIMD.
// Returns three bitmasks (bit i set means data[i] matches):
//   - quotes:      byte == '"'  (0x22)
//   - backslashes: byte == '\\' (0x5C)
//   - structural:  byte ∈ { } [ ] , : (0x7B 0x7D 0x5B 0x5D 0x2C 0x3A)
//
//go:noescape
func classify32(data *byte) (quotes, backslashes, structural uint32)

// hasAVX2 uses CPUID to check for AVX2 support.
//
//go:noescape
func hasAVX2() bool
