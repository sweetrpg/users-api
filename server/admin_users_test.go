package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
)

// TestListUsersRequiresInternalToken ports AdminUsersControllerTests and
// InternalServiceAuthTests from the Swift service: the internal-token check
// runs before any database access, so these assertions don't require a
// reachable MongoDB.
func TestListUsersRequiresInternalToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test-internal-token")
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupAdminUsersHandlers(r)

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
// INTERNAL_SERVICE_TOKEN permanently disables the route rather than trusting
// an empty presented token.
func TestListUsersDisabledWithoutConfiguredToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupAdminUsersHandlers(r)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set(internalServiceTokenHeader, "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
