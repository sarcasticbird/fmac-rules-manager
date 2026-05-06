package main

import (
	"encoding/hex"
	"testing"
)

// Real blob: neverAsk=true, userContextId=6, UUID=b022020c-1449-4a95-99a4-d8b89264d582
var blobTrueCtx6, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000010D18003601290C000000080D101C6E6576657241736B0114180200FFFF14000005404C6964656E746974794D61634164646F6E55554944012400240D20BC62303232303230632D313434392D346139352D393961342D64386238393236346435383200000000000000001300FFFF")

// Real blob: neverAsk=false, userContextId=2, UUID=4e30c912-fbd7-43ca-af2f-e6bc24fd1d90
var blobFalseCtx2, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000010D18003201290C000000080D10486E6576657241736B010000000200FFFF14000005404C6964656E746974794D61634164646F6E55554944013800240D20BC34653330633931322D666264372D343363612D616632662D65366263323466643164393000000000000000001300FFFF")

// Real blob: neverAsk=true, userContextId=10 (two digits)
var blobTrueCtx10, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000020D18043130012A08000000080D101C6E6576657241736B0116180200FFFF14000005404C6964656E746974794D61634164646F6E55554944012400240D20BC62303232303230632D313434392D346139352D393961342D64386238393236346435383200000000000000001300FFFF")

func TestExtractUserContextID(t *testing.T) {
	tests := []struct {
		name     string
		blob     []byte
		expected int
	}{
		{"single digit ctx=6", blobTrueCtx6, 6},
		{"single digit ctx=2", blobFalseCtx2, 2},
		{"two digit ctx=10", blobTrueCtx10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := extractUserContextID(tt.blob)
			if err != nil {
				t.Fatalf("extractUserContextID: %v", err)
			}
			if ctx != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, ctx)
			}
		})
	}
}

func TestExtractNeverAsk(t *testing.T) {
	tests := []struct {
		name     string
		blob     []byte
		expected bool
	}{
		{"neverAsk=true", blobTrueCtx6, true},
		{"neverAsk=false", blobFalseCtx2, false},
		{"neverAsk=true (ctx=10)", blobTrueCtx10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			na, err := extractNeverAsk(tt.blob)
			if err != nil {
				t.Fatalf("extractNeverAsk: %v", err)
			}
			if na != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, na)
			}
		})
	}
}

func TestExtractUUID(t *testing.T) {
	uuid, err := extractUUID(blobTrueCtx6)
	if err != nil {
		t.Fatalf("extractUUID: %v", err)
	}
	if uuid != "b022020c-1449-4a95-99a4-d8b89264d582" {
		t.Errorf("expected b022020c-..., got %s", uuid)
	}
}
