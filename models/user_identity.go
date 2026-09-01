// Package models defines the persisted document shapes for users-api and the
// derived views built from them.
package models

import (
	"context"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// notSoftDeletedFilter matches documents whose deletedAt field is either
// absent or explicitly null, mirroring Fluent's automatic exclusion of
// soft-deleted rows (a @Timestamp(on: .delete) field) from an unqualified
// .all() query. Both collections use the same soft-delete convention.
var notSoftDeletedFilter = bson.D{
	{Key: "$or", Value: bson.A{
		bson.D{{Key: "deletedAt", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "deletedAt", Value: nil}},
	}},
}

// userDoc is the users collection document shape. bio/website/username are additive fields
// beyond the Swift service's original Fluent schema - missing on any pre-existing document,
// which the bson driver decodes as the zero value (empty string). username is backfilled lazily
// (next login or profile edit) rather than by a batch migration; see username.go.
type userDoc struct {
	ID       uuid.UUID `bson:"_id"`
	Name     string    `bson:"name"`
	Email    string    `bson:"email"`
	Bio      string    `bson:"bio"`
	Website  string    `bson:"website"`
	Username string    `bson:"username,omitempty"`
}

// loginProfileDoc is the login_profiles collection document shape, unchanged
// from the Swift service's Fluent schema (UserModel.LoginProfile).
type loginProfileDoc struct {
	ID               uuid.UUID `bson:"_id"`
	UserID           uuid.UUID `bson:"userId"`
	ThirdPartyAuth   string    `bson:"thirdPartyAuth"`
	ThirdPartyAuthID string    `bson:"thirdPartyAuthId"`
}

// UserIdentity is a minimal id/email/subject listing for admin-web's
// role/service-access management UI to compose against auth-api's
// role/deny-entry data (auth-api holds no profile data, so it can't serve
// this itself). Deliberately narrower than the roles/denials listing that
// used to live in users-api before the auth-api split - this only ever
// returns identity, never authorization data. Subject is omitted from the
// JSON response (not null) when the user has no Auth0 login profile yet,
// matching the Swift service's synthesized Codable encoding of an Optional
// property.
type UserIdentity struct {
	ID      uuid.UUID `json:"id"`
	Email   string    `json:"email"`
	Subject *string   `json:"subject,omitempty"`
}

// ListUserIdentities returns every non-soft-deleted user's id/email, paired
// with the Auth0 subject from their login profile if one exists - the same
// join AdminUsersController.listUsers performed in Fluent.
func ListUserIdentities(ctx context.Context) ([]UserIdentity, error) {
	users, err := database.Query[userDoc](constants.UsersCollection, notSoftDeletedFilter, nil, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	profiles, err := database.Query[loginProfileDoc](constants.LoginProfilesCollection, bson.D{
		{Key: "$and", Value: bson.A{
			notSoftDeletedFilter,
			bson.D{{Key: "thirdPartyAuth", Value: constants.Auth0ThirdPartyAuth}},
		}},
	}, nil, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	subjectsByUserID := make(map[uuid.UUID]string, len(profiles))
	for _, p := range profiles {
		if _, exists := subjectsByUserID[p.UserID]; !exists {
			subjectsByUserID[p.UserID] = p.ThirdPartyAuthID
		}
	}

	identities := make([]UserIdentity, 0, len(users))
	for _, u := range users {
		identity := UserIdentity{ID: u.ID, Email: u.Email}
		if subject, ok := subjectsByUserID[u.ID]; ok {
			identity.Subject = &subject
		}
		identities = append(identities, identity)
	}
	return identities, nil
}
