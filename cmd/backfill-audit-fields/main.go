// Command backfill-audit-fields populates the platform audit fields (PADR-0001) on users-api's
// pre-convention documents: users, login_profiles, and friendships. Run once against the target
// database in a maintenance window; it is idempotent, so a re-run is a no-op.
//
// Usage:
//
//	go run ./cmd/backfill-audit-fields               # dry run - reports counts, writes nothing
//	go run ./cmd/backfill-audit-fields -apply        # perform the writes
//	go run ./cmd/backfill-audit-fields -apply -migrated-at 2026-08-01T00:00:00Z
//
// DB_URI (or the DB_* parts) is read from the environment / .env, same as the service.
package main

import (
	"context"
	"flag"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/sweetrpg/common.go/logging"
	modelcore "github.com/sweetrpg/model-core.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	apply := flag.Bool("apply", false, "perform writes (default: dry run)")
	migratedAtRaw := flag.String("migrated-at", "", "RFC3339 timestamp for created_at when no better signal exists (default: now)")
	flag.Parse()

	_ = godotenv.Load(".env")
	logging.Init()

	migratedAt := time.Now().UTC()
	if *migratedAtRaw != "" {
		parsed, err := time.Parse(time.RFC3339, *migratedAtRaw)
		if err != nil {
			logging.Logger.Error("invalid -migrated-at", "value", *migratedAtRaw, "error", err.Error())
			return
		}
		migratedAt = parsed.UTC()
	}

	database.SetupDatabase()
	defer database.TeardownDatabase()

	ctx := context.Background()
	mode := "DRY RUN"
	if *apply {
		mode = "APPLY"
	}
	logging.Logger.Info("backfill-audit-fields starting", "mode", mode, "migratedAt", migratedAt.Format(time.RFC3339))

	users := backfillUsers(ctx, *apply, migratedAt)
	profiles := backfillSimple(ctx, constants.LoginProfilesCollection, *apply, migratedAt)
	friendships := backfillFriendships(ctx, *apply)

	logging.Logger.Info("backfill-audit-fields done",
		"users", users, "login_profiles", profiles, "friendships", friendships, "mode", mode)
}

// needsCreatedAt matches a document with no usable created_at: the key is missing, null, or the
// Go/BSON zero time. This is the idempotency guard - a document already stamped is skipped.
var needsCreatedAt = bson.D{{Key: "$or", Value: bson.A{
	bson.D{{Key: "created_at", Value: bson.D{{Key: "$exists", Value: false}}}},
	bson.D{{Key: "created_at", Value: nil}},
	bson.D{{Key: "created_at", Value: time.Time{}}},
}}}

// setAudit is the $set for a system-owned backfill stamp.
func setAudit(createdAt time.Time) bson.D {
	return bson.D{
		{Key: "created_at", Value: createdAt},
		{Key: "created_by", Value: modelcore.SystemActor},
		{Key: "updated_at", Value: createdAt},
		{Key: "updated_by", Value: modelcore.SystemActor},
	}
}

// carryDeletedAt copies a legacy camelCase deletedAt onto deleted_at when the new key is absent.
// The old key is left in place (harmless, and one less thing to undo on a rollback).
func carryDeletedAt(ctx context.Context, coll string, apply bool) int {
	filter := bson.D{
		{Key: "deletedAt", Value: bson.D{{Key: "$ne", Value: nil}}},
		{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: false}}},
	}
	docs, err := findRaw(ctx, coll, filter)
	if err != nil {
		logging.Logger.Error("carryDeletedAt query failed", "collection", coll, "error", err.Error())
		return 0
	}
	n := 0
	for _, d := range docs {
		id := d.Lookup("_id")
		legacy := d.Lookup("deletedAt")
		if !apply {
			n++
			continue
		}
		_, err := database.Db.Collection(coll).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: id}},
			bson.D{{Key: "$set", Value: bson.D{{Key: "deleted_at", Value: legacy}, {Key: "deleted_by", Value: modelcore.SystemActor}}}})
		if err != nil {
			logging.Logger.Error("carryDeletedAt update failed", "collection", coll, "error", err.Error())
			continue
		}
		n++
	}
	return n
}

// backfillSimple stamps every un-stamped document in coll with migratedAt / SystemActor.
func backfillSimple(ctx context.Context, coll string, apply bool, migratedAt time.Time) int {
	carryDeletedAt(ctx, coll, apply)
	docs, err := findRaw(ctx, coll, needsCreatedAt)
	if err != nil {
		logging.Logger.Error("backfillSimple query failed", "collection", coll, "error", err.Error())
		return 0
	}
	n := 0
	for _, d := range docs {
		if !apply {
			n++
			continue
		}
		if _, err := database.Db.Collection(coll).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: d.Lookup("_id")}},
			bson.D{{Key: "$set", Value: setAudit(migratedAt)}}); err != nil {
			logging.Logger.Error("backfillSimple update failed", "collection", coll, "error", err.Error())
			continue
		}
		n++
	}
	return n
}

// backfillUsers stamps un-stamped users. created_at is the earliest friendship created_at the
// user is a party to (accounts predate their friendships), falling back to migratedAt.
func backfillUsers(ctx context.Context, apply bool, migratedAt time.Time) int {
	carryDeletedAt(ctx, constants.UsersCollection, apply)
	docs, err := findRaw(ctx, constants.UsersCollection, needsCreatedAt)
	if err != nil {
		logging.Logger.Error("backfillUsers query failed", "error", err.Error())
		return 0
	}
	n := 0
	for _, d := range docs {
		id := d.Lookup("_id")
		createdAt := earliestFriendshipTime(ctx, id, migratedAt)
		if !apply {
			n++
			continue
		}
		if _, err := database.Db.Collection(constants.UsersCollection).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: id}},
			bson.D{{Key: "$set", Value: setAudit(createdAt)}}); err != nil {
			logging.Logger.Error("backfillUsers update failed", "error", err.Error())
			continue
		}
		n++
	}
	return n
}

// earliestFriendshipTime returns the oldest created_at/createdAt among friendships the id is a
// party to, or fallback if there are none.
func earliestFriendshipTime(ctx context.Context, userID bson.RawValue, fallback time.Time) time.Time {
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "userA", Value: userID}},
		bson.D{{Key: "userB", Value: userID}},
	}}}
	docs, err := findRaw(ctx, constants.FriendshipsCollection, filter)
	if err != nil || len(docs) == 0 {
		return fallback
	}
	earliest := fallback
	found := false
	for _, d := range docs {
		for _, key := range []string{"created_at", "createdAt"} {
			if t, ok := d.Lookup(key).TimeOK(); ok {
				if !found || t.Before(earliest) {
					earliest, found = t, true
				}
			}
		}
	}
	if !found {
		return fallback
	}
	return earliest.UTC()
}

// backfillFriendships copies the legacy camelCase createdAt/updatedAt onto created_at/updated_at
// and sets created_by/updated_by to the pair's requester.
func backfillFriendships(ctx context.Context, apply bool) int {
	docs, err := findRaw(ctx, constants.FriendshipsCollection, needsCreatedAt)
	if err != nil {
		logging.Logger.Error("backfillFriendships query failed", "error", err.Error())
		return 0
	}
	n := 0
	for _, d := range docs {
		createdAt, ok := d.Lookup("createdAt").TimeOK()
		if !ok {
			continue
		}
		updatedAt, ok := d.Lookup("updatedAt").TimeOK()
		if !ok {
			updatedAt = createdAt
		}
		by := rawValueToUserID(d.Lookup("requestedBy"))
		if !apply {
			n++
			continue
		}
		if _, err := database.Db.Collection(constants.FriendshipsCollection).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: d.Lookup("_id")}},
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "created_at", Value: createdAt},
				{Key: "created_by", Value: by},
				{Key: "updated_at", Value: updatedAt},
				{Key: "updated_by", Value: by},
			}}}); err != nil {
			logging.Logger.Error("backfillFriendships update failed", "error", err.Error())
			continue
		}
		n++
	}
	return n
}

// rawValueToUserID renders a BSON value that holds a user id - binary uuid.UUID (this service's
// encoding) or a legacy plain string - as the canonical lowercase uuid string, matching what the
// live write paths stamp into created_by/updated_by. Returns "" if it can't be decoded.
func rawValueToUserID(v bson.RawValue) string {
	if s, ok := v.StringValueOK(); ok {
		if id, err := uuid.Parse(s); err == nil {
			return id.String()
		}
		return s
	}
	var id uuid.UUID
	if err := v.Unmarshal(&id); err == nil {
		return id.String()
	}
	return ""
}

func findRaw(ctx context.Context, coll string, filter bson.D) ([]bson.Raw, error) {
	cur, err := database.Db.Collection(coll).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var out []bson.Raw
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
