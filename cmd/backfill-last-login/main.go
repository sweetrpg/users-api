// Command backfill-last-login seeds login_profiles.last_login_at on rows created before that
// field existed, so users-api's admin active-user count has a value to compare against for
// pre-existing users. It is idempotent: only rows with a missing or null last_login_at are
// touched, so a re-run is a no-op.
//
// The seed is the profile's own created_at (legacy camelCase createdAt as a fallback, then
// -migrated-at / now). created_at is a stable floor: unlike updated_at it does not drift on
// profile edits, so it cannot wrongly inflate "active". Effect: a pre-existing user only counts
// as active if their login profile was created within the active-user window; otherwise they
// re-qualify on their next login, which sets last_login_at directly. A conservative undercount
// that self-corrects forward.
//
// Usage:
//
//	go run ./cmd/backfill-last-login                          # dry run - reports the count, writes nothing
//	go run ./cmd/backfill-last-login -apply                   # perform the writes
//	go run ./cmd/backfill-last-login -apply -migrated-at 2026-08-01T00:00:00Z
//
// DB_URI (or the DB_* parts) is read from the environment / .env, same as the service.
package main

import (
	"context"
	"flag"
	"time"

	"github.com/joho/godotenv"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// needsLastLogin matches a login profile with no usable last_login_at - the idempotency guard.
var needsLastLogin = bson.D{{Key: "$or", Value: bson.A{
	bson.D{{Key: "last_login_at", Value: bson.D{{Key: "$exists", Value: false}}}},
	bson.D{{Key: "last_login_at", Value: nil}},
}}}

func main() {
	apply := flag.Bool("apply", false, "perform writes (default: dry run)")
	migratedAtRaw := flag.String("migrated-at", "", "RFC3339 fallback timestamp when a profile has no created_at (default: now)")
	flag.Parse()

	_ = godotenv.Load(".env")
	logging.Init()

	fallback := time.Now().UTC()
	if *migratedAtRaw != "" {
		parsed, err := time.Parse(time.RFC3339, *migratedAtRaw)
		if err != nil {
			logging.Logger.Error("invalid -migrated-at", "value", *migratedAtRaw, "error", err.Error())
			return
		}
		fallback = parsed.UTC()
	}

	database.SetupDatabase()
	defer database.TeardownDatabase()

	ctx := context.Background()
	mode := "DRY RUN"
	if *apply {
		mode = "APPLY"
	}
	logging.Logger.Info("backfill-last-login starting", "mode", mode, "fallback", fallback.Format(time.RFC3339))

	cur, err := database.Db.Collection(constants.LoginProfilesCollection).Find(ctx, needsLastLogin)
	if err != nil {
		logging.Logger.Error("query failed", "error", err.Error())
		return
	}
	var docs []bson.Raw
	if err := cur.All(ctx, &docs); err != nil {
		logging.Logger.Error("cursor read failed", "error", err.Error())
		return
	}

	n := 0
	for _, d := range docs {
		seed := fallback
		for _, key := range []string{"created_at", "createdAt"} {
			if t, ok := d.Lookup(key).TimeOK(); ok {
				seed = t.UTC()
				break
			}
		}
		if !*apply {
			n++
			continue
		}
		if _, err := database.Db.Collection(constants.LoginProfilesCollection).UpdateOne(ctx,
			bson.D{{Key: "_id", Value: d.Lookup("_id")}},
			bson.D{{Key: "$set", Value: bson.D{{Key: "last_login_at", Value: seed}}}}); err != nil {
			logging.Logger.Error("update failed", "error", err.Error())
			continue
		}
		n++
	}

	logging.Logger.Info("backfill-last-login done", "login_profiles_backfilled", n, "mode", mode)
}
