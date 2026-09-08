package models

import (
	"context"
	"time"

	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CountUsers returns the number of non-soft-deleted user documents.
func CountUsers(ctx context.Context) (int64, error) {
	return database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, notDeletedFilter)
}

// CountNewUsers returns the number of non-soft-deleted user documents created within window
// ending now - i.e. users with created_at >= now-window.
func CountNewUsers(ctx context.Context, window time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-window)
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "created_at", Value: bson.D{{Key: "$gte", Value: cutoff}}}},
	}}}
	return database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, filter)
}

// DailyNewUsers returns, per UTC calendar day in [start, end), the count of non-soft-deleted
// users whose created_at falls in that day, keyed by "2006-01-02". Days with no new users are
// absent from the map.
func DailyNewUsers(ctx context.Context, start, end time.Time) (map[string]int64, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "$and", Value: bson.A{
			notDeletedFilter,
			bson.D{{Key: "created_at", Value: bson.D{
				{Key: "$gte", Value: start.UTC()},
				{Key: "$lt", Value: end.UTC()},
			}}},
		}}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$dateToString", Value: bson.D{
				{Key: "format", Value: "%Y-%m-%d"},
				{Key: "date", Value: "$created_at"},
				{Key: "timezone", Value: "UTC"},
			}}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cur, err := database.Db.Collection(constants.UsersCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var rows []struct {
		Day   string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Day] = r.Count
	}
	return out, nil
}

// CountUsersCreatedBefore returns the number of non-soft-deleted users created strictly before
// cutoff.
func CountUsersCreatedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "created_at", Value: bson.D{{Key: "$lt", Value: cutoff.UTC()}}}},
	}}}
	return database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, filter)
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
