package sub

import "testing"

func TestSingBoxXHTTPUsesVersionAwareSessionNames(t *testing.T) {
	settings := map[string]any{
		"sessionPlacement":   "path",
		"sessionIDPlacement": "cookie",
		"xPaddingBytes":      "80-600",
	}
	legacy := map[string]any{}
	copySingBoxXHTTP(legacy, settings, "26.6.27")
	if legacy["session_placement"] != "path" {
		t.Fatalf("legacy xhttp = %#v", legacy)
	}
	if _, exists := legacy["session_id_placement"]; exists {
		t.Fatalf("legacy output contains current field: %#v", legacy)
	}

	modern := map[string]any{}
	copySingBoxXHTTP(modern, settings, "26.9.8")
	if modern["session_id_placement"] != "cookie" {
		t.Fatalf("modern xhttp = %#v", modern)
	}
	if modern["x_padding_bytes"] != "80-600" {
		t.Fatalf("padding missing: %#v", modern)
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
	applySingBoxStream(outbound, stream, "26.9.8")
	tls := outbound["tls"].(map[string]any)
	if tls["server_name"] != "example.com" {
		t.Fatalf("tls = %#v", tls)
	}
	if tls["utls"].(map[string]any)["fingerprint"] != "chrome" {
		t.Fatalf("utls = %#v", tls["utls"])
	}
	finalMask := outbound["final_mask"].(map[string]any)
	if len(finalMask["tcp"].([]any)) != 1 {
		t.Fatalf("final_mask = %#v", finalMask)
	}
}
