package service

import "testing"

func TestParseLeafCertSHA256UsesSNIResult(t *testing.T) {
	first := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	second := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	output := "Pinging without SNI\nCert's leaf SHA256:\t" + first +
		"\nPinging with SNI\nCert's leaf SHA256:  " + second + "\n"

	got, err := parseLeafCertSHA256(output)
	if err != nil {
		t.Fatalf("parseLeafCertSHA256() error = %v", err)
	}
	if got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("parseLeafCertSHA256() = %q", got)
	}
}

func TestValidateTLSPingTarget(t *testing.T) {
	for _, target := range []string{"example.com", "example.com:8443", "[2001:db8::1]:443"} {
		if _, err := validateTLSPingTarget(target); err != nil {
			t.Errorf("validateTLSPingTarget(%q) error = %v", target, err)
		}
	}
	for _, target := range []string{"", "-ip", "https://example.com", "example.com/path", "example.com:70000"} {
		if _, err := validateTLSPingTarget(target); err == nil {
			t.Errorf("validateTLSPingTarget(%q) unexpectedly succeeded", target)
		}
	}
}
