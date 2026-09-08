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

// seedUserCreatedAt inserts a user document whose created_at is set to createdAt (rather than
// now), so the CountNewUsers window can be exercised at boundary ages.
func seedUserCreatedAt(t *testing.T, email string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	u := userDoc{ID: id, Name: "New Users Fixture", Email: email}
	modelcore.StampCreate(&u.Auditable, id.String(), createdAt.UTC())
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(), u); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
	return id
}

func TestCountNewUsers_WindowAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	const window = 7 * 24 * time.Hour

	before, err := CountNewUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountNewUsers baseline: %v", err)
	}

	// Inside the 7-day window - counted.
	seedUserCreatedAt(t, "new-users-3d-"+t.Name()+"@example.com", time.Now().UTC().Add(-3*24*time.Hour))
	// Outside the window - not counted.
	seedUserCreatedAt(t, "new-users-10d-"+t.Name()+"@example.com", time.Now().UTC().Add(-10*24*time.Hour))
	// Inside the window but soft-deleted - not counted.
	deleted := seedUserCreatedAt(t, "new-users-del-"+t.Name()+"@example.com", time.Now().UTC().Add(-24*time.Hour))
	now := time.Now().UTC()
	if _, err := database.Db.Collection(constants.UsersCollection).UpdateOne(ctx,
		bson.D{{Key: "_id", Value: deleted}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "deleted_at", Value: now}}}}); err != nil {
		t.Fatalf("soft-delete fixture: %v", err)
	}

	after, err := CountNewUsers(ctx, window)
	if err != nil {
		t.Fatalf("CountNewUsers after: %v", err)
	}
	if got := after - before; got != 1 {
		t.Errorf("new user delta = %d, want 1 (3d-old counted; 10d-old and soft-deleted excluded)", got)
	}
}
