package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
)

func newProfileTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupProfileHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

func TestGetProfile_RejectsMissingBearerToken(t *testing.T) {
	r := newProfileTestRouter(t, "")

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetProfile_RejectsInvalidToken(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized)
	r := newProfileTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateProfile_RejectsMissingBearerToken(t *testing.T) {
	r := newProfileTestRouter(t, "")

	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestUpdateProfile_ValidationRunsForAnyAuthenticatedRole proves 1.2's "no role requirement" -
// an editor-role token (not admin) reaches validation rather than being 403'd, unlike
// GET /admin/users. Uses an invalid body so the test fails at validation, before touching
// MongoDB - these assertions don't require a reachable database.
func TestUpdateProfile_ValidationRunsForAnyAuthenticatedRole(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleEditor}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newProfileTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(`{"bio":"`+strings.Repeat("x", bioMaxLength+1)+`"}`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (validation error, not 403 - proves no role requirement)", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateProfile_RejectsInvalidWebsite(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newProfileTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(`{"website":"not-a-url"}`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateProfile_AcceptsValidWebsite(t *testing.T) {
	if got := validateProfileUpdate(updateProfileRequest{Website: "https://example.com/ada"}); got != "" {
		t.Errorf("validateProfileUpdate = %q, want empty for a valid https URL", got)
	}
}

func TestUpdateProfile_RejectsRequestWithoutSchemeHost(t *testing.T) {
	cases := []string{"example.com", "ftp://example.com", "http://", "javascript:alert(1)"}
	for _, website := range cases {
		if got := validateProfileUpdate(updateProfileRequest{Website: website}); got == "" {
			t.Errorf("validateProfileUpdate(%q) = empty, want a validation error", website)
		}
	}
}
