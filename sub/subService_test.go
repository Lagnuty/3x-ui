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
					"password": "secret",
					"profiles": []any{map[string]any{
						"username":          "player_1",
						"uuid":              "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
						"texturesValue":     "texture",
						"texturesSignature": "signature",
					}},
				},
			},
		},
	}

	masks := normalizedFinalMaskTCPMasks(finalmask)

	if len(masks) != 1 {
		t.Fatalf("masks len = %d, want 1", len(masks))
	}
	settings := masks[0].(map[string]any)["settings"].(map[string]any)
	if settings["password"] != "secret" || len(settings["profiles"].([]any)) != 1 {
		t.Fatalf("xmc settings = %#v, want complete settings preserved", settings)
	}
}

func TestNormalizedFinalMaskUDPMasksMigratesLatestCoreShapes(t *testing.T) {
	finalmask := map[string]any{"udp": []any{
		map[string]any{"type": "mkcp-aes128gcm", "settings": map[string]any{"password": "secret"}},
		map[string]any{"type": "header-dtls", "settings": map[string]any{}},
		map[string]any{"type": "xicmp", "settings": map[string]any{"ip": "1.1.1.1", "id": 7}},
	}}
	masks := normalizedFinalMaskUDPMasks(finalmask)
	if len(masks) != 3 {
		t.Fatalf("masks = %#v", masks)
	}
	first := masks[0].(map[string]any)
	if first["type"] != "mkcp-legacy" || first["settings"].(map[string]any)["value"] != "secret" {
		t.Fatalf("aes migration = %#v", first)
	}
	second := masks[1].(map[string]any)
	if second["settings"].(map[string]any)["header"] != "dtls" {
		t.Fatalf("header migration = %#v", second)
	}
	third := masks[2].(map[string]any)
	if len(third["settings"].(map[string]any)["ips"].([]any)) != 1 {
		t.Fatalf("xicmp migration = %#v", third)
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

func TestBuildXhttpExtraIncludesXMUXAndSkipsOldDefaults(t *testing.T) {
	extra := buildXhttpExtra(map[string]any{
		"xPaddingBytes":        "80-600",
		"scMinPostsIntervalMs": "30",
		"xmux": map[string]any{
			"maxConcurrency":   "8-12",
			"maxConnections":   0,
			"cMaxReuseTimes":   0,
			"hMaxRequestTimes": "600-900",
		},
	})

	if extra["xPaddingBytes"] != "80-600" {
		t.Fatalf("xPaddingBytes = %v, want 80-600", extra["xPaddingBytes"])
	}
	if _, ok := extra["scMinPostsIntervalMs"]; ok {
		t.Fatalf("scMinPostsIntervalMs default should be skipped: %#v", extra)
	}
	if _, ok := extra["xmux"].(map[string]any); !ok {
		t.Fatalf("xmux should be exported: %#v", extra)
	}
}

func TestBuildXhttpExtraIncludesHeaders(t *testing.T) {
	headers := map[string]any{
		"User-Agent": "Pinned browser identity",
		"X-Trace":    "outbound",
	}
	extra := buildXhttpExtra(map[string]any{"headers": headers})

	got, ok := extra["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %#v, want a non-empty map", extra["headers"])
	}
	if got["User-Agent"] != "Pinned browser identity" || got["X-Trace"] != "outbound" {
		t.Fatalf("headers were not preserved: %#v", got)
	}
}

func TestExtractKcpShareFieldsFinalMaskLegacy(t *testing.T) {
	fields := extractKcpShareFields(map[string]any{
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{
					"type":     "mkcp-legacy",
					"settings": map[string]any{"header": "wireguard", "value": "seed"},
				},
			},
		},
	})

	if fields.headerType != "wireguard" || fields.seed != "seed" {
		t.Fatalf("legacy kcp fields = %#v, want wireguard/seed", fields)
	}
}
