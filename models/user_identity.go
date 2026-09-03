// Package models defines the persisted document shapes for users-api and the
// derived views built from them.
package models

import (
	"context"

	"github.com/google/uuid"
	modelcore "github.com/sweetrpg/model-core.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// notDeletedFilter matches documents that have not been soft-deleted: deleted_at absent or null.
// This is the platform audit-fields convention (PADR-0001 / docs/data-conventions.md). Pre-Go
// documents carry a camelCase `deletedAt` from the Swift/Fluent schema; the audit-fields backfill
// (cmd/backfill-audit-fields) copies that onto `deleted_at`, so this filter alone is sufficient
// once the backfill has run.
var notDeletedFilter = bson.D{
	{Key: "$or", Value: bson.A{
		bson.D{{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "deleted_at", Value: nil}},
	}},
}

// userDoc is the users collection document shape. bio/website/username are additive fields
// beyond the Swift service's original Fluent schema - missing on any pre-existing document,
// which the bson driver decodes as the zero value (empty string). username is backfilled lazily
// (next login or profile edit) rather than by a batch migration; see username.go. The embedded
// modelcore.Auditable holds the platform audit fields (created_at/by, updated_at/by,
// deleted_at/by); pre-Go documents get them from cmd/backfill-audit-fields.
type userDoc struct {
	ID       uuid.UUID `bson:"_id"`
	Name     string    `bson:"name"`
	Email    string    `bson:"email"`
	Bio      string    `bson:"bio"`
	Website  string    `bson:"website"`
	Username string    `bson:"username,omitempty"`

	modelcore.Auditable `bson:",inline"`
}

// loginProfileDoc is the login_profiles collection document shape (Swift Fluent's
// UserModel.LoginProfile), plus the embedded platform audit fields.
type loginProfileDoc struct {
	ID               uuid.UUID `bson:"_id"`
	UserID           uuid.UUID `bson:"userId"`
	ThirdPartyAuth   string    `bson:"thirdPartyAuth"`
	ThirdPartyAuthID string    `bson:"thirdPartyAuthId"`

	modelcore.Auditable `bson:",inline"`
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
	users, err := database.Query[userDoc](constants.UsersCollection, notDeletedFilter, nil, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	profiles, err := database.Query[loginProfileDoc](constants.LoginProfilesCollection, bson.D{
		{Key: "$and", Value: bson.A{
			notDeletedFilter,
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
