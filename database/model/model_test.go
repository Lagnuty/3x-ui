package model

import (
	"encoding/json"
	"testing"
)

func TestNormalizeStreamSettingsRenamesLegacyXHTTPSessionKeys(t *testing.T) {
	stream := `{"network":"xhttp","xhttpSettings":{"path":"/","sessionPlacement":"header","sessionKey":"x_session"}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	xhttp := got["xhttpSettings"].(map[string]any)

	if xhttp["sessionIDPlacement"] != "header" {
		t.Fatalf("sessionIDPlacement = %v, want header", xhttp["sessionIDPlacement"])
	}
	if xhttp["sessionIDKey"] != "x_session" {
		t.Fatalf("sessionIDKey = %v, want x_session", xhttp["sessionIDKey"])
	}
	if _, ok := xhttp["sessionPlacement"]; ok {
		t.Fatal("legacy sessionPlacement should be removed")
	}
	if _, ok := xhttp["sessionKey"]; ok {
		t.Fatal("legacy sessionKey should be removed")
	}
}

func TestNormalizeStreamSettingsKeepsNewXHTTPSessionKeys(t *testing.T) {
	stream := `{"network":"xhttp","xhttpSettings":{"path":"/","sessionPlacement":"header","sessionIDPlacement":"query","sessionKey":"old","sessionIDKey":"new"}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	xhttp := got["xhttpSettings"].(map[string]any)

	if xhttp["sessionIDPlacement"] != "query" {
		t.Fatalf("sessionIDPlacement = %v, want query", xhttp["sessionIDPlacement"])
	}
	if xhttp["sessionIDKey"] != "new" {
		t.Fatalf("sessionIDKey = %v, want new", xhttp["sessionIDKey"])
	}
}

func TestNormalizeInboundSettingsDropsWireGuardWorkers(t *testing.T) {
	settings := `{"secretKey":"key","workers":2,"num_workers":4,"peers":[]}`

	normalized := normalizeInboundSettings(WireGuard, settings)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized settings is invalid JSON: %v", err)
	}
	if _, ok := got["workers"]; ok {
		t.Fatal("workers should be removed")
	}
	if _, ok := got["num_workers"]; ok {
		t.Fatal("num_workers should be removed")
	}
	if got["secretKey"] != "key" {
		t.Fatalf("secretKey = %v, want key", got["secretKey"])
	}
}

func TestNormalizeInboundSettingsCoversXray26930TunAndWireGuard(t *testing.T) {
	wireguard := normalizeInboundSettings(WireGuard, `{"workers":2,"domainStrategy":"ForceIP","remoteDNS":["local","1.1.1.1"]}`)
	var wg map[string]any
	if err := json.Unmarshal([]byte(wireguard), &wg); err != nil {
		t.Fatal(err)
	}
	if _, exists := wg["domainStrategy"]; exists {
		t.Fatalf("removed WireGuard domainStrategy survived: %s", wireguard)
	}
	if got := wg["remoteDNS"].([]any); len(got) != 1 || got[0] != "1.1.1.1" {
		t.Fatalf("remoteDNS = %#v", got)
	}

	tun := normalizeInboundSettings(TUN, `{"MTU":[1500,1280],"autoSystemDNS":true}`)
	var tunSettings map[string]any
	if err := json.Unmarshal([]byte(tun), &tunSettings); err != nil {
		t.Fatal(err)
	}
	if tunSettings["mtu"] != float64(1500) || tunSettings["autoSystemDnsToGateway"] != true {
		t.Fatalf("TUN settings = %#v", tunSettings)
	}
}

func TestNormalizeStreamSettingsDropsIncompleteFinalMaskXMC(t *testing.T) {
	stream := `{"network":"tcp","finalmask":{"tcp":[{"type":"xmc","settings":{"profile":"chrome","texture":"","signature":"sig"}},{"type":"fragment","settings":{"packets":"tlshello"}}]}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	finalmask := got["finalmask"].(map[string]any)
	tcp := finalmask["tcp"].([]any)
	if len(tcp) != 1 {
		t.Fatalf("tcp masks len = %d, want 1", len(tcp))
	}
	mask := tcp[0].(map[string]any)
	if mask["type"] != "fragment" {
		t.Fatalf("remaining mask type = %v, want fragment", mask["type"])
	}
}

func TestNormalizeStreamSettingsDropsInvalidFinalMaskXMCProfile(t *testing.T) {
	stream := `{"network":"tcp","finalmask":{"tcp":[{"type":"xmc","settings":{"password":"secret","profiles":[{"username":"x","uuid":"invalid","texturesValue":"tex","texturesSignature":"sig"}]}}]}}`
	normalized := normalizeStreamSettings(stream)
	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatal(err)
	}
	if _, exists := got["finalmask"]; exists {
		t.Fatalf("invalid XMC survived normalization: %s", normalized)
	}
}

func TestNormalizeStreamSettingsKeepsCompleteFinalMaskXMC(t *testing.T) {
	stream := `{"network":"tcp","finalmask":{"tcp":[{"type":"xmc","settings":{"hostname":"mc.example.com","password":"secret","profiles":[{"username":"player_1","uuid":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","texturesValue":"tex","texturesSignature":"sig"}]}}]}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	tcp := got["finalmask"].(map[string]any)["tcp"].([]any)
	settings := tcp[0].(map[string]any)["settings"].(map[string]any)
	if settings["password"] != "secret" || len(settings["profiles"].([]any)) != 1 {
		t.Fatalf("xmc settings = %#v, want complete settings preserved", settings)
	}
}

func TestNormalizeStreamSettingsMigratesLegacyFinalMaskUDPShapes(t *testing.T) {
	stream := `{"finalmask":{"udp":[{"type":"header-wireguard","settings":{}},{"type":"xicmp","settings":{"ip":"1.1.1.1","id":1}}]}}`
	normalized := normalizeStreamSettings(stream)
	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatal(err)
	}
	udp := got["finalmask"].(map[string]any)["udp"].([]any)
	if udp[0].(map[string]any)["type"] != "mkcp-legacy" {
		t.Fatalf("header mask = %#v", udp[0])
	}
	settings := udp[1].(map[string]any)["settings"].(map[string]any)
	if len(settings["ips"].([]any)) != 1 {
		t.Fatalf("xicmp = %#v", settings)
	}
}

func TestNormalizeStreamSettingsDropsXHTTPCoreDependentDefaults(t *testing.T) {
	stream := `{"network":"xhttp","xhttpSettings":{"path":"/","scMinPostsIntervalMs":"30","scMaxEachPostBytes":"1000000","xmux":{"maxConcurrency":"16-32","maxConnections":0,"cMaxReuseTimes":0,"hMaxRequestTimes":"600-900","hMaxReusableSecs":"1800-3000","hKeepAlivePeriod":0}}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	xhttp := got["xhttpSettings"].(map[string]any)
	for _, key := range []string{"scMinPostsIntervalMs", "scMaxEachPostBytes", "xmux"} {
		if _, ok := xhttp[key]; ok {
			t.Fatalf("%s should be removed from old XHTTP defaults: %#v", key, xhttp)
		}
	}
}

func TestNormalizeStreamSettingsKeepsCustomXHTTPXMUX(t *testing.T) {
	stream := `{"network":"xhttp","xhttpSettings":{"path":"/","xmux":{"maxConcurrency":"8-12","maxConnections":0,"cMaxReuseTimes":0,"hMaxRequestTimes":"600-900","hMaxReusableSecs":"1800-3000","hKeepAlivePeriod":0}}}`

	normalized := normalizeStreamSettings(stream)

	var got map[string]any
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized streamSettings is invalid JSON: %v", err)
	}
	xmux := got["xhttpSettings"].(map[string]any)["xmux"].(map[string]any)
	if xmux["maxConcurrency"] != "8-12" {
		t.Fatalf("maxConcurrency = %v, want 8-12", xmux["maxConcurrency"])
	}
}

func TestMigrateInboundConfigCoversCompatibilityFields(t *testing.T) {
	stream := `{
		"network":"xhttp",
		"xhttpSettings":{"sessionPlacement":"cookie","sessionTable":"hex","sessionLength":24},
		"realitySettings":{"minClientVer":"26.3.27"},
		"tlsSettings":{"allowInsecure":true,"verifyPeerCertByName":"verify.example.com","settings":{"pinnedPeerCertificateChainSha256":["pin"]}},
		"finalmask":{"tcp":[{"type":"xmc","settings":{"profile":"chrome"}}]}
	}`
	result := MigrateInboundConfig(VLESS, `{}`, stream)
	if !result.Changed {
		t.Fatal("migration did not report a change")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(result.StreamSettings), &got); err != nil {
		t.Fatal(err)
	}
	xhttp := got["xhttpSettings"].(map[string]any)
	if xhttp["sessionIDPlacement"] != "cookie" || xhttp["sessionIDTable"] != "hex" || xhttp["sessionIDLength"] != float64(24) {
		t.Fatalf("xhttp migration = %#v", xhttp)
	}
	if _, exists := got["realitySettings"].(map[string]any)["minClientVer"]; exists {
		t.Fatal("legacy REALITY minimum was not cleared")
	}
	tls := got["tlsSettings"].(map[string]any)
	client := tls["settings"].(map[string]any)
	if client["verifyPeerCertByName"] != "verify.example.com" {
		t.Fatalf("TLS name verification = %#v", client)
	}
	if _, exists := client["pinnedPeerCertSha256"]; !exists {
		t.Fatalf("TLS pin was not migrated: %#v", client)
	}
	if hasLegacyAllowInsecure(got) {
		t.Fatal("allowInsecure survived migration")
	}
	if _, exists := got["finalmask"]; exists {
		t.Fatal("incomplete FinalMask XMC survived migration")
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings = %#v, want TLS and FinalMask warnings", result.Warnings)
	}
}

func TestMigrateInboundConfigNormalizesLegacyHysteria2(t *testing.T) {
	settings := `{"auth":"secret"}`
	stream := `{"network":"hysteria2","hy2Settings":{"authString":"fallback","udpIdleTimeoutSec":45}}`
	result := MigrateInboundConfig(Hysteria2, settings, stream)
	if result.Protocol != Hysteria {
		t.Fatalf("protocol = %s, want hysteria", result.Protocol)
	}

	var gotSettings, gotStream map[string]any
	_ = json.Unmarshal([]byte(result.Settings), &gotSettings)
	_ = json.Unmarshal([]byte(result.StreamSettings), &gotStream)
	clients := gotSettings["clients"].([]any)
	if clients[0].(map[string]any)["auth"] != "secret" || gotSettings["version"] != float64(2) {
		t.Fatalf("hysteria settings = %#v", gotSettings)
	}
	if gotStream["network"] != "hysteria" {
		t.Fatalf("network = %v", gotStream["network"])
	}
	hy := gotStream["hysteriaSettings"].(map[string]any)
	if hy["auth"] != "fallback" || hy["udpIdleTimeout"] != float64(45) || hy["version"] != float64(2) {
		t.Fatalf("hysteria stream = %#v", hy)
	}
}

func TestMigrateInboundConfigKeepsExplicitRealityMinimum(t *testing.T) {
	stream := `{"security":"reality","realitySettings":{"minClientVer":"26.9.8"}}`
	result := MigrateInboundConfig(VLESS, `{}`, stream)
	if result.StreamSettings != stream {
		t.Fatalf("explicit minimum changed: %s", result.StreamSettings)
	}
}
