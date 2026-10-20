package service

import (
	"encoding/json"
	"testing"
)

func TestNormalizeWireGuardOutboundsDropsWorkers(t *testing.T) {
	raw := []byte(`[
		{"protocol":"wireguard","settings":{"secretKey":"key","workers":2,"num_workers":4,"domainStrategy":"ForceIP","remoteDNS":["local","1.1.1.1"]}},
		{"protocol":"freedom","settings":{"domainStrategy":"AsIs"}}
	]`)

	normalized := normalizeWireGuardOutbounds(raw)

	var outbounds []map[string]any
	if err := json.Unmarshal(normalized, &outbounds); err != nil {
		t.Fatalf("normalized outbounds is invalid JSON: %v", err)
	}
	settings := outbounds[0]["settings"].(map[string]any)
	if _, ok := settings["workers"]; ok {
		t.Fatal("workers should be removed")
	}
	if _, ok := settings["num_workers"]; ok {
		t.Fatal("num_workers should be removed")
	}
	if _, ok := settings["domainStrategy"]; ok {
		t.Fatal("domainStrategy should be removed")
	}
	dns := settings["remoteDNS"].([]any)
	if len(dns) != 1 || dns[0] != "1.1.1.1" {
		t.Fatalf("remoteDNS = %#v", dns)
	}
	if settings["secretKey"] != "key" {
		t.Fatalf("secretKey = %v, want key", settings["secretKey"])
	}
}

func TestNormalizeOutboundStreamsMigratesXDNS(t *testing.T) {
	raw := []byte(`[{"protocol":"vless","streamSettings":{"network":"kcp","finalmask":{"udp":[{"type":"xdns","settings":{"domains":[{"name":"dns.example"}],"resolvers":[{"type":"tcp","settings":{"addr":"9.9.9.9:53"}}]}}]}}}]`)
	normalized := normalizeOutboundStreams(raw)
	var outbounds []map[string]any
	if err := json.Unmarshal(normalized, &outbounds); err != nil {
		t.Fatal(err)
	}
	stream := outbounds[0]["streamSettings"].(map[string]any)
	mask := stream["finalmask"].(map[string]any)["udp"].([]any)[0].(map[string]any)
	settings := mask["settings"].(map[string]any)
	if settings["domains"].([]any)[0].(map[string]any)["names"].([]any)[0] != "dns.example" {
		t.Fatalf("domains = %#v", settings["domains"])
	}
	if settings["resolvers"].([]any)[0].(map[string]any)["addrs"].([]any)[0] != "tcp://9.9.9.9:53" {
		t.Fatalf("resolvers = %#v", settings["resolvers"])
	}
}
