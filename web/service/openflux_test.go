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

func TestOpenFluxClientCommand(t *testing.T) {
	node := &model.OpenFluxNode{
		Transport: "yandex",
		URL:       "https://disk.yandex.ru/i/FvIK9gb_V-yJWA",
		Mode:      "l4",
		Codec:     "batched",
		Debug:     1,
	}
	args := openFluxClientArgs(node)
	wantArgs := []string{
		"--role=client",
		"--mode=l4",
		"--transport=yandex",
		"--url=https://disk.yandex.ru/i/FvIK9gb_V-yJWA",
		"--codec=batched",
		"--debug=1",
	}
	if len(args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d: %#v", len(args), len(wantArgs), args)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}
	wantCommand := "openflux '--role=client' '--mode=l4' '--transport=yandex' '--url=https://disk.yandex.ru/i/FvIK9gb_V-yJWA' '--codec=batched' '--debug=1'"
	if got := openFluxCommand(args); got != wantCommand {
		t.Fatalf("command = %q, want %q", got, wantCommand)
	}
}

func TestOpenFluxInlineKeyUsesManagedFile(t *testing.T) {
	node := &model.OpenFluxNode{
		Id:            7,
		Transport:     "yandex",
		URL:           "https://disk.yandex.ru/i/FvIK9gb_V-yJWA",
		Mode:          "l4",
		Codec:         "batched",
		Debug:         1,
		EncryptionKey: "secret",
	}
	args := openFluxArgs(node)
	want := "--encryption-key-file=/etc/openflux/keys/openflux-7.key"
	if args[len(args)-1] != want {
		t.Fatalf("last arg = %q, want %q", args[len(args)-1], want)
	}
}
