package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAdminStats_RejectsMissingBearerToken(t *testing.T) {
	r := newAdminUsersTestRouter(t, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminStats_RejectsNonAdminRole(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleEditor}, Sub: "auth0|user-sub"}, http.StatusOK)
	r := newAdminUsersTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdminStats_AuthzUnavailableIs503(t *testing.T) {
	srv := newAuthzStub(t, authz.CheckResponse{}, http.StatusInternalServerError)
	r := newAdminUsersTestRouter(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestAdminStats_ReturnsCountsForAdmin needs a reachable MongoDB - skipped unless DB_URI is set,
// matching the models DB-backed tests and server/resolve_subjects_test.go.
func TestAdminStats_ReturnsCountsForAdmin(t *testing.T) {
	if os.Getenv("DB_URI") == "" {
		t.Skip("DB_URI not set, skipping DB-backed admin-stats test")
	}
	database.SetupDatabase()

	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|admin-sub"}, http.StatusOK)
	r := newAdminUsersTestRouter(t, srv.URL)

	ctx := context.Background()
	userID := uuid.New()
	email := "admin-stats-" + t.Name() + "@example.com"
	subject := "auth0|admin-stats-" + t.Name()
	recent := time.Now().UTC().Add(-24 * time.Hour)

	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: userID}, {Key: "name", Value: "Admin Stats Fixture"}, {Key: "email", Value: email},
		{Key: "created_at", Value: recent},
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(ctx, bson.D{{Key: "_id", Value: userID}})
	})
	if _, err := database.Db.Collection(constants.LoginProfilesCollection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: uuid.New()}, {Key: "userId", Value: userID},
		{Key: "thirdPartyAuth", Value: constants.Auth0ThirdPartyAuth}, {Key: "thirdPartyAuthId", Value: subject},
		{Key: "last_login_at", Value: recent},
	}); err != nil {
		t.Fatalf("seed login profile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.LoginProfilesCollection).DeleteMany(ctx, bson.D{{Key: "thirdPartyAuthId", Value: subject}})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.Number
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	total, hasTotal := body["total_users"]
	active, hasActive := body["active_users"]
	newUsers, hasNew := body["new_users"]
	if !hasTotal || !hasActive || !hasNew {
		t.Fatalf("response missing keys: %s", rec.Body.String())
	}
	totalN, _ := total.Int64()
	activeN, _ := active.Int64()
	newN, _ := newUsers.Int64()
	if totalN < 1 {
		t.Errorf("total_users = %d, want >= 1 (seeded fixture)", totalN)
	}
	if activeN < 1 {
		t.Errorf("active_users = %d, want >= 1 (seeded fixture logged in 24h ago)", activeN)
	}
	if newN < 1 {
		t.Errorf("new_users = %d, want >= 1 (seeded fixture created just now)", newN)
	}
}
