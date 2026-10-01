package sub

import "testing"

func TestNormalizedFinalMaskTCPMasksSkipsIncompleteXMC(t *testing.T) {
	finalmask := map[string]any{
		"tcp": []any{
			map[string]any{
				"type": "xmc",
				"settings": map[string]any{
					"profile":   "chrome",
					"texture":   "",
					"signature": "sig",
				},
			},
			map[string]any{
				"type": "fragment",
				"settings": map[string]any{
					"packets": "tlshello",
				},
			},
		},
	}

	masks := normalizedFinalMaskTCPMasks(finalmask)

	if len(masks) != 1 {
		t.Fatalf("masks len = %d, want 1", len(masks))
	}
	mask := masks[0].(map[string]any)
	if mask["type"] != "fragment" {
		t.Fatalf("remaining mask type = %v, want fragment", mask["type"])
	}
}

func TestNormalizedFinalMaskTCPMasksKeepsCompleteXMC(t *testing.T) {
	finalmask := map[string]any{
		"tcp": []any{
			map[string]any{
				"type": "xmc",
				"settings": map[string]any{
					"profile":   "chrome",
					"texture":   "tex",
					"signature": "sig",
				},
			},
		},
	}

	masks := normalizedFinalMaskTCPMasks(finalmask)

	if len(masks) != 1 {
		t.Fatalf("masks len = %d, want 1", len(masks))
	}
	settings := masks[0].(map[string]any)["settings"].(map[string]any)
	if settings["profile"] != "chrome" || settings["texture"] != "tex" || settings["signature"] != "sig" {
		t.Fatalf("xmc settings = %#v, want complete settings preserved", settings)
	}
}

func TestRuleAppliesToInboundSupportsCSVTags(t *testing.T) {
	if !ruleAppliesToInbound([]any{"api", "tun-in,hysteria-in"}, "hysteria-in") {
		t.Fatal("expected CSV inboundTag list to match hysteria-in")
	}
	if ruleAppliesToInbound([]any{"api", "tun-in"}, "hysteria-in") {
		t.Fatal("unexpected inboundTag match")
	}
}
