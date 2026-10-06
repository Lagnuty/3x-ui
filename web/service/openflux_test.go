package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func TestNormalizeOpenFluxNode(t *testing.T) {
	node := &model.OpenFluxNode{
		Name:      "nl2 Yandex Docs",
		ServerID:  "nl2",
		Transport: "yandex",
		URL:       "https://disk.yandex.ru/i/example",
		Mode:      "",
		Codec:     "",
		Debug:     2,
		Enabled:   true,
	}
	if err := normalizeOpenFluxNode(node); err != nil {
		t.Fatalf("normalizeOpenFluxNode returned error: %v", err)
	}
	if node.Mode != "l4" || node.Codec != "batched" {
		t.Fatalf("defaults = mode %q codec %q", node.Mode, node.Codec)
	}
}

func TestNormalizeOpenFluxNodeRejectsBadURL(t *testing.T) {
	node := &model.OpenFluxNode{Name: "bad", Transport: "boards", URL: "https://example.com/board/1", Mode: "l4", Codec: "batched"}
	if err := normalizeOpenFluxNode(node); err != ErrOpenFluxURLInvalid {
		t.Fatalf("error = %v, want ErrOpenFluxURLInvalid", err)
	}
}

func TestOpenFluxMobileCarrier(t *testing.T) {
	if got := openFluxCarrier("boards"); got != "cursor_ws" {
		t.Fatalf("boards carrier = %q", got)
	}
	if got := openFluxCarrier("vyandex"); got != "volga_ws" {
		t.Fatalf("vyandex carrier = %q", got)
	}
	if got := openFluxCarrier("yandex"); got != "yandex_docs" {
		t.Fatalf("yandex carrier = %q", got)
	}
}
