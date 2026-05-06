// host/idbkey_test.go
package main

import (
	"bytes"
	"testing"
)

func TestDecodeIDBKey(t *testing.T) {
	// Correct encoding of "/siteContainerMap@@_aistudio.google.com"
	// (byte+1 encoding: '/' -> 0x30, 's' -> 0x74, 'i' -> 0x6a, 't' -> 0x75, 'e' -> 0x66, ...)
	raw := []byte{
		0x30, 0x74, 0x6A, 0x75, 0x66, 0x44, 0x70, 0x6F,
		0x75, 0x62, 0x6A, 0x6F, 0x66, 0x73, 0x4E, 0x62,
		0x71, 0x41, 0x41, 0x60, 0x62, 0x6A, 0x74, 0x75,
		0x76, 0x65, 0x6A, 0x70, 0x2F, 0x68, 0x70, 0x70,
		0x68, 0x6D, 0x66, 0x2F, 0x64, 0x70, 0x6E,
	}
	decoded := decodeIDBKey(raw)
	expected := "/siteContainerMap@@_aistudio.google.com"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestEncodeIDBKey(t *testing.T) {
	key := "/siteContainerMap@@_github.com"
	encoded := encodeIDBKey(key)
	decoded := decodeIDBKey(encoded)
	if decoded != key {
		t.Errorf("round-trip failed: expected %q, got %q", key, decoded)
	}
}

func TestSiteContainerMapKey(t *testing.T) {
	key := siteContainerMapKey("github.com")
	decoded := decodeIDBKey(key)
	expected := "/siteContainerMap@@_github.com"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	sites := []string{"github.com", "mail.google.com", "us-east-2.signin.aws.amazon.com"}
	for _, site := range sites {
		key := siteContainerMapKey(site)
		decoded := decodeIDBKey(key)
		expected := "/siteContainerMap@@_" + site
		if decoded != expected {
			t.Errorf("site %s: expected %q, got %q", site, expected, decoded)
		}
	}
}

func TestIsSiteContainerMapKey(t *testing.T) {
	good := encodeIDBKey("/siteContainerMap@@_github.com")
	bad := encodeIDBKey("/containerTabsOpened")

	if !isSiteContainerMapKey(good) {
		t.Error("expected true for siteContainerMap key")
	}
	if isSiteContainerMapKey(bad) {
		t.Error("expected false for non-siteContainerMap key")
	}
}

func TestExtractSiteFromKey(t *testing.T) {
	key := encodeIDBKey("/siteContainerMap@@_docs.google.com")
	site := extractSiteFromKey(key)
	if site != "docs.google.com" {
		t.Errorf("expected docs.google.com, got %s", site)
	}
}

func TestEncodeMatchesReal(t *testing.T) {
	// Correct byte+1 encoding of "/siteContainerMap@@_aistudio.google.com"
	realKey := []byte{
		0x30, 0x74, 0x6A, 0x75, 0x66, 0x44, 0x70, 0x6F,
		0x75, 0x62, 0x6A, 0x6F, 0x66, 0x73, 0x4E, 0x62,
		0x71, 0x41, 0x41, 0x60, 0x62, 0x6A, 0x74, 0x75,
		0x76, 0x65, 0x6A, 0x70, 0x2F, 0x68, 0x70, 0x70,
		0x68, 0x6D, 0x66, 0x2F, 0x64, 0x70, 0x6E,
	}
	generated := siteContainerMapKey("aistudio.google.com")
	if !bytes.Equal(realKey, generated) {
		t.Errorf("generated key doesn't match real key\nreal:      %x\ngenerated: %x", realKey, generated)
	}
}
