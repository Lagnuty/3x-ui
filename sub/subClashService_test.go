package sub

import "testing"

func TestApplyRealitySecurityEnablesMLKEMAndDefaultsChrome(t *testing.T) {
	proxy := map[string]any{"type": "vless"}
	stream := map[string]any{
		"realitySettings": map[string]any{
			"publicKey": "public",
			"shortId":   "0123456789abcdef",
		},
	}

	service := &SubClashService{}
	if !service.applySecurity(proxy, "reality", stream) {
		t.Fatal("applySecurity() rejected REALITY settings")
	}
	if proxy["client-fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome", proxy["client-fingerprint"])
	}
	opts := proxy["reality-opts"].(map[string]any)
	if opts["support-x25519mlkem768"] != true {
		t.Fatalf("reality-opts = %#v, want support-x25519mlkem768: true", opts)
	}
}

func TestApplyRealitySecurityAllowsExplicitLegacyOptOut(t *testing.T) {
	proxy := map[string]any{"type": "vless"}
	stream := map[string]any{
		"realitySettings": map[string]any{
			"publicKey":             "public",
			"supportX25519MLKEM768": false,
		},
	}

	service := &SubClashService{}
	if !service.applySecurity(proxy, "reality", stream) {
		t.Fatal("applySecurity() rejected REALITY settings")
	}
	opts := proxy["reality-opts"].(map[string]any)
	if _, exists := opts["support-x25519mlkem768"]; exists {
		t.Fatalf("reality-opts = %#v, ML-KEM flag should be omitted after explicit opt-out", opts)
	}
}

func TestTLSDataPreservesVerificationOverrides(t *testing.T) {
	service := &SubClashService{}
	got := service.tlsData(map[string]any{
		"serverName": "example.com",
		"settings": map[string]any{
			"verifyPeerCertByName": "verify.example.com",
			"pinnedPeerCertSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	if got["verifyPeerCertByName"] != "verify.example.com" {
		t.Fatalf("verifyPeerCertByName = %v", got["verifyPeerCertByName"])
	}
	if got["pinnedPeerCertSha256"] == "" {
		t.Fatal("pinnedPeerCertSha256 was not preserved")
	}
}

func TestApplyTransportExportsMihomoXHTTPOptions(t *testing.T) {
	proxy := map[string]any{}
	stream := map[string]any{"xhttpSettings": map[string]any{
		"path":               "/api",
		"mode":               "packet-up",
		"sessionIDPlacement": "cookie",
		"xPaddingBytes":      "80-600",
		"headers":            map[string]any{"User-Agent": "custom"},
	}}
	service := &SubClashService{}
	if !service.applyTransport(proxy, "xhttp", stream) {
		t.Fatal("xhttp transport was rejected")
	}
	if proxy["network"] != "xhttp" {
		t.Fatalf("network = %v", proxy["network"])
	}
	opts := proxy["xhttp-opts"].(map[string]any)
	if opts["session-id-placement"] != "cookie" || opts["x-padding-bytes"] != "80-600" {
		t.Fatalf("xhttp-opts = %#v", opts)
	}
}
