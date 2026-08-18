package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
)

func newAdminUsersTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupAdminUsersHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

func newAuthzStub(t *testing.T, response authz.CheckResponse, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestListUsersRequiresInternalToken ports AdminUsersControllerTests and
// InternalServiceAuthTests from the Swift service: the legacy internal-token check runs
// before any database access, so these assertions don't require a reachable MongoDB.
func TestListUsersRequiresInternalToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test-internal-token")
	r := newAdminUsersTestRouter(t, "")

	t.Run("missing token is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("mismatched token is unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		req.Header.Set(internalServiceTokenHeader, "wrong-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

// TestListUsersDisabledWithoutConfiguredToken mirrors the Swift service's
// InternalServiceAuth.hasValidInternalServiceToken guarantee: an unset
// INTERNAL_SERVICE_TOKEN permanently disables the legacy fallback path rather than
// trusting an empty presented token.
func TestListUsersDisabledWithoutConfiguredToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	r := newAdminUsersTestRouter(t, "")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set(internalServiceTokenHeader, "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListUsersBearerToken_RejectsWithoutAdminRole(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleEditor}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newAdminUsersTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListUsersBearerToken_RejectsInvalidToken(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized)
	r := newAdminUsersTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
