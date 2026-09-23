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

func newIdentitiesTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupIdentitiesHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

func TestLinkStart_RejectsMissingBearerToken(t *testing.T) {
	r := newIdentitiesTestRouter(t, "")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/link/start", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLinkComplete_RejectsMissingBearerToken(t *testing.T) {
	r := newIdentitiesTestRouter(t, "")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/link/complete", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLinkComplete_RejectsInvalidTicketAfterAuthzSucceeds(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newIdentitiesTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/link/complete", strings.NewReader(`{"ticket":"not-a-uuid"}`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListIdentities_RejectsMissingBearerToken(t *testing.T) {
	r := newIdentitiesTestRouter(t, "")

	req := httptest.NewRequest(http.MethodGet, "/api/identities", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUnlinkIdentity_RejectsMissingBearerToken(t *testing.T) {
	r := newIdentitiesTestRouter(t, "")

	req := httptest.NewRequest(http.MethodDelete, "/api/identities/"+"00000000-0000-0000-0000-000000000000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUnlinkIdentity_RejectsInvalidIDAfterAuthzSucceeds(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newIdentitiesTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodDelete, "/api/identities/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
