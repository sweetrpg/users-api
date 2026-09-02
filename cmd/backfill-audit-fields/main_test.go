package main

import (
	"testing"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func lookup(t *testing.T, doc bson.D, key string) bson.RawValue {
	t.Helper()
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bson.Raw(raw).Lookup(key)
}

func TestRawValueToUserID(t *testing.T) {
	id := uuid.New()

	// Binary uuid.UUID - this service's own encoding.
	if got := rawValueToUserID(lookup(t, bson.D{{Key: "v", Value: id}}, "v")); got != id.String() {
		t.Errorf("binary uuid: got %q, want %q", got, id.String())
	}
	// Legacy lowercase string.
	if got := rawValueToUserID(lookup(t, bson.D{{Key: "v", Value: id.String()}}, "v")); got != id.String() {
		t.Errorf("string uuid: got %q, want %q", got, id.String())
	}
	// Legacy uppercase string (Swift UUID.uuidString) - normalized to lowercase.
	upper := "B3384F5D-0000-0000-0000-000000000000"
	if got := rawValueToUserID(lookup(t, bson.D{{Key: "v", Value: upper}}, "v")); got != "b3384f5d-0000-0000-0000-000000000000" {
		t.Errorf("uppercase uuid: got %q, want lowercased", got)
	}
	// Non-uuid value - empty.
	if got := rawValueToUserID(lookup(t, bson.D{{Key: "v", Value: 42}}, "v")); got != "" {
		t.Errorf("non-uuid: got %q, want empty", got)
	}
}
