package models

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrProfileNotFound means the verified subject has no User/LoginProfile pair yet - the caller
// hasn't completed a login since provisioning existed, or provisioning failed for their most
// recent login (see add-users-api-provisioning's design.md).
var ErrProfileNotFound = errors.New("models: profile not found")

// ErrUsernameTaken means a profile update tried to set a username another user already holds
// (the sparse unique index on users.username rejected the write).
var ErrUsernameTaken = errors.New("models: username already taken")

// Profile is the self-service view of a User's own record.
type Profile struct {
	UserID   uuid.UUID
	Name     string
	Email    string
	Bio      string
	Website  string
	Username string
}

// userIDFilter matches _id against this service's own binary uuid.UUID encoding and both cases
// of a legacy plain-string UUID - see decodeFlexibleUUID's doc comment in provisioning.go. A
// User document predating this Go service (a manually bootstrapped admin record, confirmed in
// dev) can have `_id` stored as a plain BSON string - specifically *uppercase*, matching Swift's
// `UUID.uuidString` convention (the original Fluent-era service this data predates). A bare
// equality filter with a binary uuid.UUID value, or even a lowercase string, never matches such
// a document: confirmed live, where the stored `_id` was "B3384F5D-..." while this service's own
// `uuid.UUID.String()` produces "b3384f5d-...". MongoDB string comparison is case-sensitive, so
// both cases need to be listed explicitly.
func userIDFilter(id uuid.UUID) bson.D {
	lower := id.String()
	return bson.D{{Key: "_id", Value: bson.D{
		{Key: "$in", Value: bson.A{id, lower, strings.ToUpper(lower)}},
	}}}
}

// FindProfileBySubject resolves a verified Auth0 subject to its own Profile, via the same
// LoginProfile join FindOrCreateUser uses. Returns ErrProfileNotFound if the subject has never
// been provisioned.
func FindProfileBySubject(ctx context.Context, subject string) (*Profile, error) {
	loginProfile, err := findLoginProfileBySubject(ctx, subject)
	if err != nil {
		logging.Logger.Warn("findLoginProfileBySubject errored", "subject", subject, "error", err.Error())
		return nil, err
	}
	if loginProfile == nil {
		logging.Logger.Warn("step1: no LoginProfile found for subject", "subject", subject)
		return nil, ErrProfileNotFound
	}
	logging.Logger.Info("step1: LoginProfile found", "subject", subject, "userId", loginProfile.UserID.String())

	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		userIDFilter(loginProfile.UserID),
	}}}
	var raw bson.Raw
	err = database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logging.Logger.Warn("step2: no User doc matched userIDFilter", "subject", subject, "userId", loginProfile.UserID.String())
			return nil, ErrProfileNotFound
		}
		logging.Logger.Warn("step2: User lookup errored", "subject", subject, "userId", loginProfile.UserID.String(), "error", err.Error())
		return nil, err
	}
	logging.Logger.Info("step2: User doc found", "subject", subject, "userId", loginProfile.UserID.String())
	id, err := decodeFlexibleUUID(raw.Lookup("_id"))
	if err != nil {
		return nil, fmt.Errorf("decoding legacy user _id for subject %q: %w", subject, err)
	}
	name, _ := raw.Lookup("name").StringValueOK()
	email, _ := raw.Lookup("email").StringValueOK()
	bio, _ := raw.Lookup("bio").StringValueOK()
	website, _ := raw.Lookup("website").StringValueOK()
	username, _ := raw.Lookup("username").StringValueOK()

	return &Profile{UserID: id, Name: name, Email: email, Bio: bio, Website: website, Username: username}, nil
}

// UpdateProfile updates name/bio/website/username for userID - email is never writable through
// this path (see design.md's "email is read-only" decision). Returns ErrProfileNotFound if
// userID doesn't match an existing, non-soft-deleted User, or ErrUsernameTaken if username
// collides with another user (the sparse unique index rejects the write).
func UpdateProfile(ctx context.Context, userID uuid.UUID, name, bio, website, username string) error {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		userIDFilter(userID),
	}}}
	set := bson.D{
		{Key: "name", Value: name},
		{Key: "bio", Value: bio},
		{Key: "website", Value: website},
	}
	// Only touch username when a value is given. Writing "" would land an empty string on the
	// sparse unique index (which excludes only *missing* keys, not empty ones), so two cleared
	// usernames would collide.
	if username != "" {
		set = append(set, bson.E{Key: "username", Value: username})
	}
	update := bson.D{{Key: "$set", Value: set}}

	result, err := database.Db.Collection(constants.UsersCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrUsernameTaken
		}
		return err
	}
	if result.MatchedCount == 0 {
		return ErrProfileNotFound
	}
	return nil
}
