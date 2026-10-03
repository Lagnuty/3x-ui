package service

import "testing"

func TestLinuxCapabilityFlags(t *testing.T) {
	netAdmin, netRaw := linuxCapabilityFlags("Name:\tx-ui\nCapEff:\t0000000000003000\n")
	if !netAdmin || !netRaw {
		t.Fatalf("capabilities = netAdmin:%t netRaw:%t, want both", netAdmin, netRaw)
	}
	netAdmin, netRaw = linuxCapabilityFlags("CapEff:\t0000000000001000\n")
	if !netAdmin || netRaw {
		t.Fatalf("capabilities = netAdmin:%t netRaw:%t, want only NET_ADMIN", netAdmin, netRaw)
	}
}
