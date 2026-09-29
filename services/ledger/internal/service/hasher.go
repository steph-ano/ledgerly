package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// CanonicalHash generates a deterministic SHA-256 hex string from an arbitrary struct or map.
// It normalizes by deserializing and re-serializing through a generic map structure with sorted keys.
func CanonicalHash(payload any) (string, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload for hashing: %w", err)
	}

	var normalized any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return "", fmt.Errorf("failed to unmarshal for canonical sorting: %w", err)
	}

	canonicalBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to produce canonical json: %w", err)
	}

	sum := sha256.Sum256(canonicalBytes)
	return hex.EncodeToString(sum[:]), nil
}
