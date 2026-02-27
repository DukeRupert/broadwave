package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateAPIKey returns a full key (bw_ + 32 hex chars = 35 chars total)
// and its display prefix (first 11 chars: bw_ + 8 hex).
func generateAPIKey() (fullKey, prefix string, err error) {
	b := make([]byte, 16) // 128 bits
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating api key: %w", err)
	}
	fullKey = "bw_" + hex.EncodeToString(b)
	prefix = fullKey[:11]
	return fullKey, prefix, nil
}
