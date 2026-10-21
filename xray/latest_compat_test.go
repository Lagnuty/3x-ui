package xray

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLatestXrayCompatibility validates the representative v26.6-v26.10
// feature contract against a real Xray binary when XRAY_LATEST_BINARY is set.
func TestLatestXrayCompatibility(t *testing.T) {
	binary := strings.TrimSpace(os.Getenv("XRAY_LATEST_BINARY"))
	if binary == "" {
		t.Skip("XRAY_LATEST_BINARY is not set")
	}
	for _, name := range []string{"xray-v26.10.17-compat.json", "xray-v26.10.21-filter.json"} {
		config := filepath.Join("..", "testdata", name)
		output, err := exec.Command(binary, "run", "-test", "-c", config).CombinedOutput()
		if err != nil {
			t.Fatalf("latest Xray rejected %s: %v\n%s", name, err, output)
		}
	}
}
