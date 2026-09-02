package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/common.go/logging"
	modelcore "github.com/sweetrpg/model-core.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProvisionResult is the outcome of FindOrCreateUser.
type ProvisionResult struct {
	UserID  uuid.UUID
	Created bool
}

// EnsureLoginProfileIndexes creates the unique index on
// login_profiles.(thirdPartyAuth, thirdPartyAuthId) that FindOrCreateUser relies on to detect a
// concurrent first-login race via a duplicate-key error instead of a read-then-write check. Safe
// to call on every startup - CreateOne is idempotent for an identical index definition.
func EnsureLoginProfileIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.LoginProfilesCollection)
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "thirdPartyAuth", Value: 1},
			{Key: "thirdPartyAuthId", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// EnsureUserIndexes creates the unique index on users.email - inherited from the original Swift
// service's Fluent schema (a `.unique(on: \.$email)` field constraint), but never explicitly
// created by this Go service until now, relying entirely on the index having survived the
// migration. Codified here so FindOrCreateUser's email-collision handling has something to
// actually collide with in a fresh database (a new local/test Mongo instance, or if the
// collection is ever recreated) rather than depending on inherited Atlas state. Safe to call on
// every startup - CreateOne is idempotent for an identical index definition.
func EnsureUserIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.UsersCollection)
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	// Sparse so the many pre-existing user docs without a username (backfilled lazily, see
	// username.go) don't all collide on a null key.
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	})
	return err
}

// FindOrCreateUser returns the User.id linked to subject's Auth0 login, creating a new User and
// LoginProfile the first time that subject is seen. name/email seed the new User doc on creation
// only - a repeat call for an existing subject never overwrites them (profile edits are a
// separate change's concern, see design.md).
func FindOrCreateUser(ctx context.Context, subject, name, email string) (ProvisionResult, error) {
	if profile, err := findLoginProfileBySubject(ctx, subject); err != nil {
		return ProvisionResult{}, err
	} else if profile != nil {
		// Backfill a username for an existing user that predates the username field. Best
		// effort - a failure here shouldn't block the login.
		if err := EnsureUsername(ctx, profile.UserID, usernameSeed(email, name)); err != nil {
			logging.Logger.Warn("username backfill failed", "userId", profile.UserID.String(), "error", err.Error())
		}
		return ProvisionResult{UserID: profile.UserID, Created: false}, nil
	}

	userID := uuid.New()
	username, err := GenerateUsername(ctx, usernameSeed(email, name))
	if err != nil {
		return ProvisionResult{}, err
	}
	// A user provisioning their own account on first login is the acting user for that write.
	user := userDoc{ID: userID, Name: name, Email: email, Username: username}
	modelcore.StampCreate(&user.Auditable, userID.String(), time.Now().UTC())
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(ctx, user); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return ProvisionResult{}, err
		}
		// A User with this email already exists - predates this subject's first Auth0 login
		// (e.g. a manually bootstrapped admin record, see auth-api's AGENTS.md for the
		// equivalent user_roles bootstrap procedure). Adopt it rather than failing: link a new
		// LoginProfile to the existing User instead of erroring on the collision. `Created`
		// below still reflects whether this call is the one that newly links this *subject* -
		// not whether the User document itself was newly inserted, matching the documented API
		// contract ("repeat calls for an existing subject return created: false").
		existingUser, findErr := findUserByEmail(ctx, email)
		if findErr != nil {
			return ProvisionResult{}, findErr
		}
		if existingUser == nil {
			return ProvisionResult{}, err
		}
		userID = existingUser.ID
		if ensureErr := EnsureUsername(ctx, userID, usernameSeed(email, name)); ensureErr != nil {
			logging.Logger.Warn("username backfill failed for adopted user", "userId", userID.String(), "error", ensureErr.Error())
		}
	}

	profile := loginProfileDoc{
		ID:               uuid.New(),
		UserID:           userID,
		ThirdPartyAuth:   constants.Auth0ThirdPartyAuth,
		ThirdPartyAuthID: subject,
	}
	modelcore.StampCreate(&profile.Auditable, userID.String(), time.Now().UTC())
	_, err = database.Db.Collection(constants.LoginProfilesCollection).InsertOne(ctx, profile)
	if err == nil {
		return ProvisionResult{UserID: userID, Created: true}, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		// ponytail: the userDoc insert above already committed, so a non-duplicate-key
		// failure here leaves an orphaned User with no LoginProfile. Acceptable for now -
		// revisit with a transaction if orphaned Users become a real problem.
		return ProvisionResult{}, err
	}

	// Lost the race: a concurrent first login for the same subject won. Its LoginProfile now
	// exists, so return its User.id instead of the one we just created.
	existing, findErr := findLoginProfileBySubject(ctx, subject)
	if findErr != nil {
		return ProvisionResult{}, findErr
	}
	if existing == nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{UserID: existing.UserID, Created: false}, nil
}

// findUserByEmail decodes leniently via bson.Raw rather than straight into userDoc: a legacy
// document predating this Go service (e.g. a manually bootstrapped admin record, see auth-api's
// AGENTS.md for the equivalent user_roles bootstrap procedure) can have `_id` stored as a plain
// BSON string rather than this service's own binary uuid.UUID encoding - confirmed in dev, where
// a real such document exists. A straight `.Decode(&userDoc{})` errors on that mismatch instead
// of adopting the record.
func findUserByEmail(ctx context.Context, email string) (*userDoc, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "email", Value: email}},
	}}}

	var raw bson.Raw
	err := database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	id, err := decodeFlexibleUUID(raw.Lookup("_id"))
	if err != nil {
		return nil, fmt.Errorf("decoding legacy user _id for email %q: %w", email, err)
	}

	name, _ := raw.Lookup("name").StringValueOK()
	docEmail, _ := raw.Lookup("email").StringValueOK()
	return &userDoc{ID: id, Name: name, Email: docEmail}, nil
}

// decodeFlexibleUUID accepts both this service's own binary uuid.UUID encoding and a legacy
// plain-string UUID.
func decodeFlexibleUUID(v bson.RawValue) (uuid.UUID, error) {
	if s, ok := v.StringValueOK(); ok {
		return uuid.Parse(s)
	}
	var id uuid.UUID
	if err := v.Unmarshal(&id); err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func findLoginProfileBySubject(ctx context.Context, subject string) (*loginProfileDoc, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "thirdPartyAuth", Value: constants.Auth0ThirdPartyAuth}},
		bson.D{{Key: "thirdPartyAuthId", Value: subject}},
	}}}

	var profile loginProfileDoc
	err := database.Db.Collection(constants.LoginProfilesCollection).FindOne(ctx, filter).Decode(&profile)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}
