package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
)

func newProvisionTestRouter(t *testing.T, configuredToken string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/internal/identities/provision", requireInternalServiceToken(configuredToken), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestRequireInternalServiceToken_RejectsMissingHeader(t *testing.T) {
	r := newProvisionTestRouter(t, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireInternalServiceToken_RejectsWrongToken(t *testing.T) {
	r := newProvisionTestRouter(t, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	req.Header.Set(internalServiceTokenHeader, "wrong")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireInternalServiceToken_RejectsWhenUnconfigured(t *testing.T) {
	r := newProvisionTestRouter(t, "")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	req.Header.Set(internalServiceTokenHeader, "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireInternalServiceToken_AllowsMatchingToken(t *testing.T) {
	r := newProvisionTestRouter(t, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{}`))
	req.Header.Set(internalServiceTokenHeader, "s3cret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProvisionHandler_RejectsMissingSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logging.Init()
	r := gin.New()
	r.POST("/internal/identities/provision", requireInternalServiceToken("s3cret"), provisionHandler())

	req := httptest.NewRequest(http.MethodPost, "/internal/identities/provision", strings.NewReader(`{"name":"a"}`))
	req.Header.Set(internalServiceTokenHeader, "s3cret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
