package service

import "testing"

func TestXrayVersionNumber(t *testing.T) {
	tests := map[string]int{
		"v26.4.25": 26_004_025,
		"26.9.8":   26_009_008,
		"v26.9.30": 26_009_030,
		"invalid":  0,
	}
	for input, want := range tests {
		if got := xrayVersionNumber(input); got != want {
			t.Fatalf("xrayVersionNumber(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestContainsLegacyAllowInsecure(t *testing.T) {
	config := map[string]any{
		"streamSettings": map[string]any{
			"tlsSettings": map[string]any{"allowInsecure": false},
		},
	}
	if !containsLegacyAllowInsecure(config) {
		t.Fatal("nested allowInsecure field was not detected")
	}
	if containsLegacyAllowInsecure(map[string]any{"tlsSettings": map[string]any{"serverName": "example.com"}}) {
		t.Fatal("configuration without allowInsecure was reported as legacy")
	}
}
