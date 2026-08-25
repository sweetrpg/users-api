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

func newProvisionTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupProvisionHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

func TestProvision_RejectsMissingBearerToken(t *testing.T) {
	r := newProvisionTestRouter(t, "")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProvision_RejectsInvalidToken(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized)
	r := newProvisionTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProvision_RejectsWhenAuthzDenies(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: false, Reason: "service_denied"}, http.StatusOK)
	r := newProvisionTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProvision_RejectsInvalidBodyAfterAuthzSucceeds(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newProvisionTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
