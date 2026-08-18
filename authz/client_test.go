package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sweetrpg/common.go/logging"
)

func init() {
	logging.Init()
}

func TestCheckAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer good-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer good-token")
		}
		_ = json.NewEncoder(w).Encode(CheckResponse{Allowed: true, Roles: []string{RoleAdmin}, Sub: "auth0|123"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	resp, err := client.Check(context.Background(), "good-token", "users-api")
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !resp.Allowed {
		t.Error("Allowed = false, want true")
	}
	if resp.Sub != "auth0|123" {
		t.Errorf("Sub = %q, want %q", resp.Sub, "auth0|123")
	}
}

func TestCheckInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Check(context.Background(), "bad-token", "users-api")
	if _, ok := err.(InvalidTokenError); !ok {
		t.Fatalf("Check() error = %v (%T), want InvalidTokenError", err, err)
	}
}

func TestCheckDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(CheckResponse{Allowed: false, Reason: "service_denied"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	resp, err := client.Check(context.Background(), "denied-token", "users-api")
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if resp.Allowed {
		t.Error("Allowed = true, want false")
	}
}

func TestCheckBackendUnreachable(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	_, err := client.Check(context.Background(), "any-token", "users-api")
	if err == nil {
		t.Fatal("Check() error = nil, want a transport error")
	}
	if _, ok := err.(InvalidTokenError); ok {
		t.Fatal("Check() returned InvalidTokenError for an unreachable backend, want a plain error")
	}
}
