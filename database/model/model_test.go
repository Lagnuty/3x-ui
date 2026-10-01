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
