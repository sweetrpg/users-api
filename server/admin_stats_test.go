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

func TestAdminStatsHistory_RejectsMissingBearerToken(t *testing.T) {
	r := newAdminUsersTestRouter(t, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/stats/history", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// statsHistoryEntry mirrors the JSON one element of GET /admin/stats/history.
type statsHistoryEntry struct {
	Date       string `json:"date"`
	TotalUsers int64  `json:"total_users"`
	NewUsers   int64  `json:"new_users"`
}

// TestAdminStatsHistory_* need a reachable MongoDB - skipped unless DB_URI is set, matching the
// other DB-backed admin-stats tests.
func TestAdminStatsHistory_SeriesShapeAndClamping(t *testing.T) {
	if os.Getenv("DB_URI") == "" {
		t.Skip("DB_URI not set, skipping DB-backed admin-stats-history test")
	}
	database.SetupDatabase()

	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|admin-sub"}, http.StatusOK)
	r := newAdminUsersTestRouter(t, srv.URL)

	get := func(path string) (*httptest.ResponseRecorder, []statsHistoryEntry) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var body []statsHistoryEntry
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %q: %v", rec.Body.String(), err)
			}
		}
		return rec, body
	}

	todayKey := time.Now().UTC().Format("2006-01-02")

	rec, body := get("/admin/stats/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("default: status = %d, body %s", rec.Code, rec.Body.String())
	}
	if len(body) != 30 {
		t.Errorf("default series length = %d, want 30", len(body))
	}
	if len(body) > 0 && body[len(body)-1].Date != todayKey {
		t.Errorf("last entry date = %s, want today %s", body[len(body)-1].Date, todayKey)
	}
	// total_users is a cumulative count - it must never decrease day over day.
	var prev int64 = -1
	for i, e := range body {
		if e.TotalUsers < prev {
			t.Errorf("total_users decreased at index %d: %d < %d", i, e.TotalUsers, prev)
		}
		prev = e.TotalUsers
	}

	if rec, body := get("/admin/stats/history?days=1"); rec.Code != http.StatusOK || len(body) != 1 {
		t.Errorf("days=1: status %d len %d, want 200/1", rec.Code, len(body))
	}
	if rec, body := get("/admin/stats/history?days=0"); rec.Code != http.StatusOK || len(body) != 1 {
		t.Errorf("days=0 clamps to 1: status %d len %d", rec.Code, len(body))
	}
	if rec, body := get("/admin/stats/history?days=500"); rec.Code != http.StatusOK || len(body) != 90 {
		t.Errorf("days=500 clamps to 90: status %d len %d", rec.Code, len(body))
	}
	if rec, _ := get("/admin/stats/history?days=notanint"); rec.Code != http.StatusBadRequest {
		t.Errorf("days=notanint: status %d, want 400", rec.Code)
	}
}

func TestAdminStatsHistory_CountsANewUserOnItsDay(t *testing.T) {
	if os.Getenv("DB_URI") == "" {
		t.Skip("DB_URI not set, skipping DB-backed admin-stats-history test")
	}
	database.SetupDatabase()

	srv := newAuthzStub(t, authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|admin-sub"}, http.StatusOK)
	r := newAdminUsersTestRouter(t, srv.URL)

	ctx := context.Background()
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	threeDaysAgo := today.AddDate(0, 0, -3).Add(9 * time.Hour)

	userID := uuid.New()
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: userID},
		{Key: "name", Value: "History Fixture"},
		{Key: "email", Value: "history-" + t.Name() + "@example.com"},
		{Key: "created_at", Value: threeDaysAgo},
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(ctx, bson.D{{Key: "_id", Value: userID}})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/stats/history?days=7", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var body []statsHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	wantDay := threeDaysAgo.Format("2006-01-02")
	sawDay := false
	for i, e := range body {
		if e.Date != wantDay {
			continue
		}
		sawDay = true
		if e.NewUsers < 1 {
			t.Errorf("new_users on %s = %d, want >= 1", wantDay, e.NewUsers)
		}
		// total_users is >= 1 on that day and every day after it.
		for _, later := range body[i:] {
			if later.TotalUsers < 1 {
				t.Errorf("total_users on %s = %d, want >= 1 (>= the seeded user's day)", later.Date, later.TotalUsers)
			}
		}
	}
	if !sawDay {
		t.Fatalf("day %s not in the 7-day series: %+v", wantDay, body)
	}
}

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
