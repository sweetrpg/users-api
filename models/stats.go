package models

import (
	"context"
	"time"

	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// CountUsers returns the number of non-soft-deleted user documents.
func CountUsers(ctx context.Context) (int64, error) {
	return database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, notDeletedFilter)
}

// CountActiveUsers returns the number of distinct users whose most recent login falls within
// window ending now - i.e. login_profiles with last_login_at >= now-window, deduplicated by
// userId so a user with several login profiles counts once. Soft-deleted profiles are excluded.
// A user with no last_login_at (never logged in, or a pre-backfill row) is not counted.
func CountActiveUsers(ctx context.Context, window time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-window)
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "last_login_at", Value: bson.D{{Key: "$gte", Value: cutoff}}}},
	}}}
	values, err := database.Db.Collection(constants.LoginProfilesCollection).Distinct(ctx, "userId", filter)
	if err != nil {
		return 0, err
	}
	return int64(len(values)), nil
}
