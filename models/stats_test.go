package models

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	modelcore "github.com/sweetrpg/model-core.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// seedStatsUser inserts a non-soft-deleted user document and returns its id.
func seedStatsUser(t *testing.T, email string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	u := userDoc{ID: id, Name: "Stats Fixture", Email: email}
	modelcore.StampCreate(&u.Auditable, id.String(), time.Now().UTC())
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(), u); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
	return id
}

// seedLoginProfile inserts a login profile for userID. When lastLogin is non-nil it is stored as
// last_login_at; a nil lastLogin models a user who has never logged in since the field existed.
func seedLoginProfile(t *testing.T, userID uuid.UUID, subject string, lastLogin *time.Time) {
	t.Helper()
	p := loginProfileDoc{
		ID:               uuid.New(),
		UserID:           userID,
		ThirdPartyAuth:   constants.Auth0ThirdPartyAuth,
		ThirdPartyAuthID: subject,
		LastLoginAt:      lastLogin,
	}
	modelcore.StampCreate(&p.Auditable, userID.String(), time.Now().UTC())
	if _, err := database.Db.Collection(constants.LoginProfilesCollection).InsertOne(context.Background(), p); err != nil {
		t.Fatalf("seed login profile for %s: %v", subject, err)
	}
	t.Cleanup(func() { cleanupSubject(t, subject) })
}

func TestCountActiveUsers_WindowBoundaries(t *testing.T) {
	ctx := context.Background()
	const window = 30 * 24 * time.Hour

	totalBefore, err := CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers baseline: %v", err)
	}
	activeBefore, err := CountActiveUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountActiveUsers baseline: %v", err)
	}

	tenDaysAgo := time.Now().UTC().Add(-10 * 24 * time.Hour)
	fortyFiveDaysAgo := time.Now().UTC().Add(-45 * 24 * time.Hour)

	recent := seedStatsUser(t, "stats-recent-"+t.Name()+"@example.com")
	seedLoginProfile(t, recent, "auth0|stats-recent-"+t.Name(), &tenDaysAgo)

	stale := seedStatsUser(t, "stats-stale-"+t.Name()+"@example.com")
	seedLoginProfile(t, stale, "auth0|stats-stale-"+t.Name(), &fortyFiveDaysAgo)

	neverLoggedIn := seedStatsUser(t, "stats-never-"+t.Name()+"@example.com")
	seedLoginProfile(t, neverLoggedIn, "auth0|stats-never-"+t.Name(), nil)

	totalAfter, err := CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers after: %v", err)
	}
	activeAfter, err := CountActiveUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountActiveUsers after: %v", err)
	}

	if got := totalAfter - totalBefore; got != 3 {
		t.Errorf("total user delta = %d, want 3 (all three fixtures counted in total)", got)
	}
	if got := activeAfter - activeBefore; got != 1 {
		t.Errorf("active user delta = %d, want 1 (only the 10-day-old login is within the 30-day window)", got)
	}
}

func TestCountActiveUsers_DeduplicatesByUser(t *testing.T) {
	ctx := context.Background()
	const window = 30 * 24 * time.Hour

	activeBefore, err := CountActiveUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountActiveUsers baseline: %v", err)
	}

	recent := time.Now().UTC().Add(-time.Hour)
	user := seedStatsUser(t, "stats-dedup-"+t.Name()+"@example.com")
	seedLoginProfile(t, user, "auth0|stats-dedup-a-"+t.Name(), &recent)
	seedLoginProfile(t, user, "auth0|stats-dedup-b-"+t.Name(), &recent)

	activeAfter, err := CountActiveUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountActiveUsers after: %v", err)
	}
	if got := activeAfter - activeBefore; got != 1 {
		t.Errorf("active user delta = %d, want 1 (two login profiles for one user count once)", got)
	}
}
