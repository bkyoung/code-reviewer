package determinism

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// GenerateSeed creates a deterministic uint64 seed from base and target refs.
// The seed is derived from a SHA-256 hash of the concatenated refs, ensuring
// reproducibility for the same inputs.
func GenerateSeed(baseRef, targetRef string) uint64 {
	// Concatenate refs with a delimiter to ensure unique combinations
	input := fmt.Sprintf("%s|%s", baseRef, targetRef)

	// Hash the input
	hash := sha256.Sum256([]byte(input))

	// Convert the first 8 bytes of the hash to uint64
	seed := binary.BigEndian.Uint64(hash[:8])

	return seed
}
