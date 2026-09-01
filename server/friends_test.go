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

func newFriendsTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupFriendsHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

// friendsRoutes lists every route the friends collection registers, so the auth-gate
// assertions below run against all of them.
var friendsRoutes = []struct {
	method string
	path   string
}{
	{http.MethodGet, "/friends"},
	{http.MethodGet, "/friends/requests"},
	{http.MethodPost, "/friends/requests"},
	{http.MethodPost, "/friends/requests/2f1c4d6e-0000-4000-8000-000000000001/accept"},
	{http.MethodPost, "/friends/requests/2f1c4d6e-0000-4000-8000-000000000001/decline"},
	{http.MethodDelete, "/friends/2f1c4d6e-0000-4000-8000-000000000001"},
}

func TestFriends_RejectMissingBearerToken(t *testing.T) {
	r := newFriendsTestRouter(t, "")

	for _, route := range friendsRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestFriends_RejectInvalidToken(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized)
	r := newFriendsTestRouter(t, srv.URL)

	for _, route := range friendsRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("Authorization", "Bearer bad-token")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

// TestSendFriendRequest_RejectsEmptyIdentifier proves the send handler validates the request
// body shape after the authz check but before any MongoDB access, so this assertion needs no
// reachable database. A non-empty identifier would go on to a DB lookup, so it isn't covered
// here - see models/friendship_test.go's ResolveFriendTarget cases.
func TestSendFriendRequest_RejectsEmptyIdentifier(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newFriendsTestRouter(t, srv.URL)

	cases := map[string]string{
		"empty identifier":   `{"identifier":""}`,
		"missing identifier": `{}`,
		"malformed json":     `{`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/friends/requests", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer good-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}
