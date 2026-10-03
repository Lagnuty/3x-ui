package sub

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLatestSingBoxCompatibility(t *testing.T) {
	binary := os.Getenv("SING_BOX_LATEST_BINARY")
	if binary == "" {
		t.Skip("SING_BOX_LATEST_BINARY is not set")
	}
	config := filepath.Join("..", "testdata", "sing-box-v1.14.1-compat.json")
	output, err := exec.Command(binary, "check", "-c", config).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box rejected compatibility fixture: %v\n%s", err, output)
	}
}

func TestLatestMihomoCompatibility(t *testing.T) {
	binary := os.Getenv("MIHOMO_LATEST_BINARY")
	if binary == "" {
		t.Skip("MIHOMO_LATEST_BINARY is not set")
	}
	config := filepath.Join("..", "testdata", "mihomo-v1.19.32-compat.yaml")
	output, err := exec.Command(binary, "-t", "-f", config).CombinedOutput()
	if err != nil {
		t.Fatalf("Mihomo rejected compatibility fixture: %v\n%s", err, output)
	}
}
