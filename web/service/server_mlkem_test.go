package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseMLKEM768Output(t *testing.T) {
	seed := base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	client := base64.RawURLEncoding.EncodeToString(make([]byte, 1184))

	got, err := parseMLKEM768Output("Client: " + client + "\nSeed: " + seed + "\n")
	if err != nil {
		t.Fatalf("parseMLKEM768Output() error = %v", err)
	}
	if got["seed"] != seed || got["client"] != client {
		t.Fatalf("parseMLKEM768Output() = %#v", got)
	}
}

func TestParseMLKEM768OutputRejectsTruncatedKey(t *testing.T) {
	seed := base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	client := base64.RawURLEncoding.EncodeToString(make([]byte, 32))

	_, err := parseMLKEM768Output("Seed: " + seed + "\nClient: " + client)
	if err == nil || !strings.Contains(err.Error(), "client has decoded length") {
		t.Fatalf("parseMLKEM768Output() error = %v, want client length error", err)
	}
}

func TestValidateMLKEM768SeedRejectsInvalidBase64(t *testing.T) {
	err := validateMLKEM768Value("seed", "not-base64", 64)
	if err == nil || !strings.Contains(err.Error(), "not valid base64") {
		t.Fatalf("validateMLKEM768Value() error = %v, want base64 error", err)
	}
}
