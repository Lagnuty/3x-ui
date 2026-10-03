package xray

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLatestXrayCompatibility validates the representative v26.6-v26.9
// feature contract against a real Xray binary when XRAY_LATEST_BINARY is set.
func TestLatestXrayCompatibility(t *testing.T) {
	binary := strings.TrimSpace(os.Getenv("XRAY_LATEST_BINARY"))
	if binary == "" {
		t.Skip("XRAY_LATEST_BINARY is not set")
	}
	config := filepath.Join("..", "testdata", "xray-v26.9.9-compat.json")
	output, err := exec.Command(binary, "run", "-test", "-c", config).CombinedOutput()
	if err != nil {
		t.Fatalf("latest Xray rejected compatibility config: %v\n%s", err, output)
	}
}
