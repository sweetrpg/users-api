package models

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrProfileNotFound means the verified subject has no User/LoginProfile pair yet - the caller
// hasn't completed a login since provisioning existed, or provisioning failed for their most
// recent login (see add-users-api-provisioning's design.md).
var ErrProfileNotFound = errors.New("models: profile not found")

// Profile is the self-service view of a User's own record.
type Profile struct {
	UserID  uuid.UUID
	Name    string
	Email   string
	Bio     string
	Website string
}

// FindProfileBySubject resolves a verified Auth0 subject to its own Profile, via the same
// LoginProfile join FindOrCreateUser uses. Returns ErrProfileNotFound if the subject has never
// been provisioned.
func FindProfileBySubject(ctx context.Context, subject string) (*Profile, error) {
	loginProfile, err := findLoginProfileBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if loginProfile == nil {
		return nil, ErrProfileNotFound
	}

	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "_id", Value: loginProfile.UserID}},
	}}}
	var user userDoc
	err = database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}

	return &Profile{
		UserID: user.ID, Name: user.Name, Email: user.Email, Bio: user.Bio, Website: user.Website,
	}, nil
}

// UpdateProfile updates name/bio/website for userID - email is never writable through this
// path (see design.md's "email is read-only" decision). Returns ErrProfileNotFound if userID
// doesn't match an existing, non-soft-deleted User.
func UpdateProfile(ctx context.Context, userID uuid.UUID, name, bio, website string) error {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "_id", Value: userID}},
	}}}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "name", Value: name},
		{Key: "bio", Value: bio},
		{Key: "website", Value: website},
	}}}

	result, err := database.Db.Collection(constants.UsersCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrProfileNotFound
	}
	return nil
}
