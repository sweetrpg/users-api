package models

import (
	"context"
	"errors"

	"github.com/google/uuid"
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
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
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
		return ProvisionResult{UserID: profile.UserID, Created: false}, nil
	}

	userID := uuid.New()
	user := userDoc{ID: userID, Name: name, Email: email}
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
	}

	profile := loginProfileDoc{
		ID:               uuid.New(),
		UserID:           userID,
		ThirdPartyAuth:   constants.Auth0ThirdPartyAuth,
		ThirdPartyAuthID: subject,
	}
	_, err := database.Db.Collection(constants.LoginProfilesCollection).InsertOne(ctx, profile)
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

func findUserByEmail(ctx context.Context, email string) (*userDoc, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "email", Value: email}},
	}}}

	var user userDoc
	err := database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func findLoginProfileBySubject(ctx context.Context, subject string) (*loginProfileDoc, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
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
