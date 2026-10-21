package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestEmbeddedStreamSettingsIncludesXDriveEditor(t *testing.T) {
	server := NewServer()
	tpl, err := server.getHtmlTemplate(template.FuncMap{
		"i18n": func(key string, _ ...string) string { return key },
	})
	if err != nil {
		t.Fatalf("parse embedded templates: %v", err)
	}
	if tpl.Lookup("form/streamXDrive") == nil {
		t.Fatal("XDRIVE editor template was not embedded")
	}
	var rendered bytes.Buffer
	if err := tpl.ExecuteTemplate(&rendered, "form/streamSettings", nil); err != nil {
		t.Fatalf("render stream settings: %v", err)
	}
	if !strings.Contains(rendered.String(), "XDRIVE (v26.10.17+)") {
		t.Fatal("XDRIVE option missing from rendered stream settings")
	}
}

func TestEmbeddedFinalMaskIncludesSourceIPFilter(t *testing.T) {
	server := NewServer()
	tpl, err := server.getHtmlTemplate(template.FuncMap{
		"i18n": func(key string, _ ...string) string { return key },
	})
	if err != nil {
		t.Fatalf("parse embedded templates: %v", err)
	}
	var rendered bytes.Buffer
	if err := tpl.ExecuteTemplate(&rendered, "form/streamFinalMask", nil); err != nil {
		t.Fatalf("render FinalMask settings: %v", err)
	}
	for _, field := range []string{"Source IP filter (v26.10.21+)", "allowedSourceIPs", "blockedSourceIPs"} {
		if !strings.Contains(rendered.String(), field) {
			t.Fatalf("FinalMask editor is missing %q", field)
		}
	}
}
