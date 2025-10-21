package determinism

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// GenerateSeed creates a deterministic uint64 seed from base and target refs.
// The seed is derived from a SHA-256 hash of the concatenated refs, ensuring
// reproducibility for the same inputs.
// The returned value is guaranteed to be <= math.MaxInt64 (9223372036854775807)
// to ensure compatibility with LLM APIs that use signed int64 for seeds.
func GenerateSeed(baseRef, targetRef string) uint64 {
	// Concatenate refs with a delimiter to ensure unique combinations
	input := fmt.Sprintf("%s|%s", baseRef, targetRef)

	// Hash the input
	hash := sha256.Sum256([]byte(input))

	// Convert the first 8 bytes of the hash to uint64
	seed := binary.BigEndian.Uint64(hash[:8])

	// Mask off the high bit to ensure the value fits in int64
	// This keeps the seed in range [0, 9223372036854775807] (math.MaxInt64)
	seed = seed & 0x7FFFFFFFFFFFFFFF

	return seed
}
