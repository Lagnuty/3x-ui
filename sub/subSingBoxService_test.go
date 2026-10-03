package sub

import "testing"

func TestSingBoxRejectsUnsupportedXrayExtensions(t *testing.T) {
	if err := singBoxCompatibilityError(map[string]any{"network": "xhttp"}); err == nil {
		t.Fatal("XHTTP was accepted for official sing-box")
	}
	if err := singBoxCompatibilityError(map[string]any{
		"network": "tcp",
		"finalmask": map[string]any{"tcp": []any{
			map[string]any{"type": "fragment", "settings": map[string]any{"packets": "tlshello"}},
		}},
	}); err == nil {
		t.Fatal("FinalMask was accepted for official sing-box")
	}
	if err := singBoxCompatibilityError(map[string]any{"network": "kcp"}); err == nil {
		t.Fatal("mKCP was accepted for official sing-box")
	}
	if err := singBoxCompatibilityError(map[string]any{"network": "grpc"}); err != nil {
		t.Fatalf("gRPC was rejected: %v", err)
	}
}

func TestSingBoxStreamExportsRealityAndNormalizedFinalMask(t *testing.T) {
	outbound := map[string]any{}
	stream := map[string]any{
		"security": "reality",
		"realitySettings": map[string]any{
			"serverNames": []any{"example.com"},
			"shortIds":    []any{"abcd"},
			"settings": map[string]any{
				"publicKey": "public",
			},
		},
		"finalmask": map[string]any{"tcp": []any{
			map[string]any{"type": "xmc", "settings": map[string]any{"profile": "chrome"}},
			map[string]any{"type": "fragment", "settings": map[string]any{"packets": "tlshello"}},
		}},
	}
	applySingBoxStream(outbound, stream)
	tls := outbound["tls"].(map[string]any)
	if tls["server_name"] != "example.com" {
		t.Fatalf("tls = %#v", tls)
	}
	if tls["utls"].(map[string]any)["fingerprint"] != "chrome" {
		t.Fatalf("utls = %#v", tls["utls"])
	}
	if _, exists := outbound["final_mask"]; exists {
		t.Fatalf("unsupported final_mask leaked into sing-box: %#v", outbound)
	}
}
