package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func newResolveSubjectsTestRouter(t *testing.T, authzBaseURL string) *gin.Engine {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	setupResolveSubjectsHandlers(r, authz.NewClient(authzBaseURL))
	return r
}

func TestResolveSubjects_RejectsWithoutAdminAuth(t *testing.T) {
	t.Run("missing credentials is unauthorized", func(t *testing.T) {
		r := newResolveSubjectsTestRouter(t, "")

		req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`{"subjects":["auth0|x"]}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("non-bearer authorization header is unauthorized", func(t *testing.T) {
		r := newResolveSubjectsTestRouter(t, "")

		req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`{"subjects":["auth0|x"]}`))
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid bearer token is unauthorized", func(t *testing.T) {
		srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusUnauthorized)
		r := newResolveSubjectsTestRouter(t, srv.URL)

		req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`{"subjects":["auth0|x"]}`))
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("non-admin role is forbidden", func(t *testing.T) {
		srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: "auth0|user-sub"}, http.StatusOK)
		r := newResolveSubjectsTestRouter(t, srv.URL)

		req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`{"subjects":["auth0|x"]}`))
		req.Header.Set("Authorization", "Bearer good-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

func TestResolveSubjects_RejectsInvalidBodyAfterAuthzSucceeds(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|admin-sub"}, http.StatusOK)
	r := newResolveSubjectsTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestResolveSubjects_ResolvesKnownAndOmitsUnknown asserts the handler maps a known Auth0
// subject to its users._id (via the login_profiles join) and omits subjects with no resolvable
// user. Requires a reachable MongoDB like the models DB-backed tests - skipped unless DB_URI is
// set (CI provides it).
func TestResolveSubjects_ResolvesKnownAndOmitsUnknown(t *testing.T) {
	if os.Getenv("DB_URI") == "" {
		t.Skip("DB_URI not set, skipping DB-backed resolve-subjects test")
	}
	logging.Init()
	database.SetupDatabase()
	t.Cleanup(database.TeardownDatabase)

	ctx := t.Context()
	knownSubject := "auth0|known-sub-" + t.Name()
	unknownSubject := "auth0|unknown-sub-" + t.Name()
	userID := uuid.New()
	profileID := uuid.New()

	_, err := database.Db.Collection(constants.UsersCollection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: userID},
		{Key: "name", Value: "Resolve Subjects"},
		{Key: "email", Value: "resolvesubjects@example.com"},
	})
	requireNoError(t, err)
	_, err = database.Db.Collection(constants.LoginProfilesCollection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: profileID},
		{Key: "userId", Value: userID},
		{Key: "thirdPartyAuth", Value: constants.Auth0ThirdPartyAuth},
		{Key: "thirdPartyAuthId", Value: knownSubject},
	})
	requireNoError(t, err)
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(ctx, bson.D{{Key: "_id", Value: userID}})
		_, _ = database.Db.Collection(constants.LoginProfilesCollection).DeleteOne(ctx, bson.D{{Key: "_id", Value: profileID}})
	})

	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|admin-sub"}, http.StatusOK)
	r := newResolveSubjectsTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/internal/resolve-subjects", strings.NewReader(`{"subjects":["`+knownSubject+`","`+unknownSubject+`"]}`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
		return
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("resolved count = %d, want 1: %v", len(got), got)
	}
	if got[knownSubject] != userID.String() {
		t.Errorf("resolved[%q] = %q, want %q", knownSubject, got[knownSubject], userID.String())
	}
	if _, ok := got[unknownSubject]; ok {
		t.Errorf("unknown subject %q was resolved, want omitted", unknownSubject)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
