package main

import (
	"bytes"
	"fmt"
	"strconv"
)

var (
	markerUserContextID        = []byte("userContextId")
	markerNeverAsk             = []byte("neverAsk")
	markerIdentityMacAddonUUID = []byte("identityMacAddonUUID")
)

func extractUserContextID(blob []byte) (int, error) {
	pos := bytes.Index(blob, markerUserContextID)
	if pos == -1 {
		return 0, fmt.Errorf("userContextId marker not found")
	}

	i := pos + len(markerUserContextID)
	for i < len(blob) && (blob[i] < 0x30 || blob[i] > 0x39) {
		i++
	}

	var digits []byte
	for i < len(blob) && blob[i] >= 0x30 && blob[i] <= 0x39 {
		digits = append(digits, blob[i])
		i++
	}

	if len(digits) == 0 {
		return 0, fmt.Errorf("no digits found after userContextId")
	}

	val, err := strconv.Atoi(string(digits))
	if err != nil {
		return 0, fmt.Errorf("parsing userContextId digits: %w", err)
	}
	return val, nil
}

func extractNeverAsk(blob []byte) (bool, error) {
	pos := bytes.Index(blob, markerNeverAsk)
	if pos == -1 {
		return false, fmt.Errorf("neverAsk marker not found")
	}

	// After "neverAsk" text, skip 1 byte (0x01), then check the value byte.
	// true encoding:  01 14/16 ...   (byte after 0x01 is > 0x00)
	// false encoding: 01 00 00 00 ...
	valOffset := pos + len(markerNeverAsk) + 1
	if valOffset >= len(blob) {
		return false, fmt.Errorf("neverAsk value out of bounds")
	}

	return blob[valOffset] != 0x00, nil
}

func extractUUID(blob []byte) (string, error) {
	pos := bytes.Index(blob, markerIdentityMacAddonUUID)
	if pos == -1 {
		return "", fmt.Errorf("identityMacAddonUUID marker not found")
	}

	// Scan forward from the marker to find a UUID pattern (36 chars: 8-4-4-4-12 hex)
	searchStart := pos + len(markerIdentityMacAddonUUID)
	for i := searchStart; i < len(blob)-36; i++ {
		candidate := string(blob[i : i+36])
		if isUUIDFormat(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("UUID not found after identityMacAddonUUID marker")
}

func isUUIDFormat(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
