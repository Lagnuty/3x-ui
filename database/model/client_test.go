package model

import (
	"encoding/json"
	"testing"
)

func TestClientUnmarshalJSONFlexibleTgID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "empty string", raw: `{"email":"a","tgId":""}`, want: 0},
		{name: "numeric string", raw: `{"email":"a","tgId":"12345"}`, want: 12345},
		{name: "number", raw: `{"email":"a","tgId":12345}`, want: 12345},
		{name: "null", raw: `{"email":"a","tgId":null}`, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client Client
			if err := json.Unmarshal([]byte(tt.raw), &client); err != nil {
				t.Fatal(err)
			}
			if client.TgID != tt.want {
				t.Fatalf("tgId mismatch: got %d want %d", client.TgID, tt.want)
			}
		})
	}
}
