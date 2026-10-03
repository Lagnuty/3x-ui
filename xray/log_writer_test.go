package xray

import (
	"testing"
	"time"
)

func TestLogWriterStartupDiagnostics(t *testing.T) {
	lw := NewLogWriter()
	lw.ResetStartup(time.Now().Add(-time.Second))
	_, _ = lw.Write([]byte("2026/10/03 12:00:00.000001 [Debug] common/geodata: geodata mph domain matcher cache MISS for 2 rules\n"))
	_, _ = lw.Write([]byte("2026/10/03 12:00:00.000002 [Debug] common/geodata: geodata mph domain matcher cache HIT for 2 rules\n"))
	_, _ = lw.Write([]byte("2026/10/03 12:00:00.000003 [Error] common/geodata: failed to load geosite.dat: invalid data\n"))
	_, _ = lw.Write([]byte("2026/10/03 12:00:00.000004 [Warning] core: Xray 26.9.8 started\n"))

	diagnostics := lw.Snapshot()
	if !diagnostics.Ready {
		t.Fatal("startup marker was not detected")
	}
	if diagnostics.CacheHits != 1 || diagnostics.CacheMisses != 1 {
		t.Fatalf("cache counters = %d/%d, want 1/1", diagnostics.CacheHits, diagnostics.CacheMisses)
	}
	if len(diagnostics.GeoErrors) != 1 {
		t.Fatalf("geo errors = %#v, want one error", diagnostics.GeoErrors)
	}
	if diagnostics.StartupDurationMs < 900 {
		t.Fatalf("startup duration = %dms, want about one second", diagnostics.StartupDurationMs)
	}
}
