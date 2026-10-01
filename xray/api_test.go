package xray

import "testing"

func TestProcessTrafficAcceptsNestedTunCounterNames(t *testing.T) {
	traffic := map[string]*Traffic{}

	processTraffic([]string{"", "inbound", "tun-in", "tun", "uplink"}, 123, traffic)
	processTraffic([]string{"", "inbound", "tun-in", "tun", "downlink"}, 456, traffic)

	got := traffic["tun-in"]
	if got == nil {
		t.Fatal("tun inbound traffic was not recorded")
	}
	if !got.IsInbound || got.IsOutbound {
		t.Fatalf("direction flags = inbound:%v outbound:%v, want inbound only", got.IsInbound, got.IsOutbound)
	}
	if got.Up != 123 || got.Down != 456 {
		t.Fatalf("traffic = up:%d down:%d, want up:123 down:456", got.Up, got.Down)
	}
}

func TestProcessTrafficAcceptsLegacyCounterNames(t *testing.T) {
	traffic := map[string]*Traffic{}

	processTraffic([]string{"", "outbound", "direct", "", "uplink"}, 10, traffic)
	processTraffic([]string{"", "outbound", "direct", "", "downlink"}, 20, traffic)

	got := traffic["direct"]
	if got == nil {
		t.Fatal("outbound traffic was not recorded")
	}
	if got.IsInbound || !got.IsOutbound {
		t.Fatalf("direction flags = inbound:%v outbound:%v, want outbound only", got.IsInbound, got.IsOutbound)
	}
	if got.Up != 10 || got.Down != 20 {
		t.Fatalf("traffic = up:%d down:%d, want up:10 down:20", got.Up, got.Down)
	}
}
