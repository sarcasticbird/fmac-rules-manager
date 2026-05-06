// host/idbkey.go
package main

import "strings"

const siteContainerMapPrefix = "/siteContainerMap@@_"

func decodeIDBKey(raw []byte) string {
	var b strings.Builder
	for _, c := range raw {
		if c > 0 {
			b.WriteByte(c - 1)
		}
	}
	return b.String()
}

func encodeIDBKey(key string) []byte {
	encoded := make([]byte, len(key))
	for i, c := range []byte(key) {
		encoded[i] = c + 1
	}
	return encoded
}

func siteContainerMapKey(site string) []byte {
	return encodeIDBKey(siteContainerMapPrefix + site)
}

func isSiteContainerMapKey(key []byte) bool {
	decoded := decodeIDBKey(key)
	return strings.HasPrefix(decoded, siteContainerMapPrefix)
}

func extractSiteFromKey(key []byte) string {
	decoded := decodeIDBKey(key)
	return strings.TrimPrefix(decoded, siteContainerMapPrefix)
}
