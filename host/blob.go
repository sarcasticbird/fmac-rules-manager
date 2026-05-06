package main

import (
	"bytes"
	"crypto/rand"
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
	for i := searchStart; i+36 <= len(blob); i++ {
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

func generateUUID() string {
	var uuid [16]byte
	rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func buildBlob(template []byte, userContextID int, neverAsk bool) ([]byte, error) {
	ctxStr := strconv.Itoa(userContextID)

	tmplCtxID, err := extractUserContextID(template)
	if err != nil {
		return nil, fmt.Errorf("reading template userContextId: %w", err)
	}
	tmplCtxStr := strconv.Itoa(tmplCtxID)

	pos := bytes.Index(template, markerUserContextID)
	i := pos + len(markerUserContextID)
	for i < len(template) && (template[i] < 0x30 || template[i] > 0x39) {
		i++
	}
	digitStart := i

	var result []byte

	if len(ctxStr) == len(tmplCtxStr) {
		result = make([]byte, len(template))
		copy(result, template)
		for j := 0; j < len(ctxStr); j++ {
			result[digitStart+j] = ctxStr[j]
		}
	} else {
		result, err = rebuildWithNewCtxID(template, digitStart, tmplCtxStr, ctxStr)
		if err != nil {
			return nil, err
		}
	}

	newUUID := generateUUID()
	uuidPos := bytes.Index(result, markerIdentityMacAddonUUID)
	if uuidPos == -1 {
		return nil, fmt.Errorf("identityMacAddonUUID marker not found in result")
	}
	searchStart := uuidPos + len(markerIdentityMacAddonUUID)
	for j := searchStart; j+36 <= len(result); j++ {
		if isUUIDFormat(string(result[j : j+36])) {
			copy(result[j:j+36], []byte(newUUID))
			break
		}
	}

	tmplNA, _ := extractNeverAsk(template)
	if tmplNA != neverAsk {
		result, err = toggleNeverAsk(result, neverAsk)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func rebuildWithNewCtxID(template []byte, digitStart int, oldCtx, newCtx string) ([]byte, error) {
	digitEnd := digitStart + len(oldCtx)

	var result bytes.Buffer
	result.Write(template[:digitStart])
	result.Write([]byte(newCtx))
	result.Write(template[digitEnd:])

	blob := result.Bytes()

	markerPos := bytes.Index(blob, markerUserContextID)
	countPos := markerPos + len(markerUserContextID) + 3
	if countPos < len(blob) {
		blob[countPos] = byte(len(newCtx))
	}

	return blob, nil
}

func toggleNeverAsk(blob []byte, neverAsk bool) ([]byte, error) {
	pos := bytes.Index(blob, markerNeverAsk)
	if pos == -1 {
		return nil, fmt.Errorf("neverAsk marker not found")
	}

	afterMarker := pos + len(markerNeverAsk)
	if afterMarker+8 > len(blob) {
		return nil, fmt.Errorf("neverAsk: blob too short")
	}
	currentTrue := blob[afterMarker+1] != 0x00

	if currentTrue == neverAsk {
		return blob, nil
	}

	if currentTrue && !neverAsk {
		trueEnd := afterMarker + 7
		var result bytes.Buffer
		result.Write(blob[:afterMarker])
		result.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0xFF, 0xFF})
		result.Write(blob[trueEnd:])
		return result.Bytes(), nil
	}

	falseEnd := afterMarker + 8
	var result bytes.Buffer
	result.Write(blob[:afterMarker])
	result.Write([]byte{0x01, 0x14, 0x18, 0x02, 0x00, 0xFF, 0xFF})
	result.Write(blob[falseEnd:])
	return result.Bytes(), nil
}

func pickTemplate(blobs [][]byte, neverAsk bool) []byte {
	for _, b := range blobs {
		na, err := extractNeverAsk(b)
		if err == nil && na == neverAsk {
			return b
		}
	}
	if len(blobs) > 0 {
		return blobs[0]
	}
	return nil
}
