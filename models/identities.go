package models

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// ErrLastLoginMethod means an unlink request would leave the caller's User with zero
// LoginProfile rows - rejected per design.md's "never zero login methods" guarantee, enforced
// here rather than solely in the users-web UI.
var ErrLastLoginMethod = errors.New("models: cannot unlink the last remaining login method")

// LinkedIdentity is the self-service view of one of the caller's linked LoginProfile rows.
type LinkedIdentity struct {
	LoginProfileID uuid.UUID
	ConnectionType string
	LinkedAt       time.Time
}

// connectionType extracts the Auth0 connection name from a sub value formatted
// "<connection>|<id>" (e.g. "github|12345", "auth0|abc123", "google-oauth2|98765"). Falls back to
// the full subject if it doesn't contain the expected separator.
func connectionType(subject string) string {
	if idx := strings.IndexByte(subject, '|'); idx >= 0 {
		return subject[:idx]
	}
	return subject
}

// ListLinkedIdentities returns userID's linked LoginProfiles (connection type, linked date).
func ListLinkedIdentities(ctx context.Context, userID uuid.UUID) ([]LinkedIdentity, error) {
	profiles, err := loginProfilesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	identities := make([]LinkedIdentity, 0, len(profiles))
	for _, p := range profiles {
		identities = append(identities, LinkedIdentity{
			LoginProfileID: p.ID,
			ConnectionType: connectionType(p.ThirdPartyAuthID),
			LinkedAt:       p.CreatedAt,
		})
	}
	return identities, nil
}

// UnlinkIdentity removes loginProfileID from userID's linked identities. Returns
// ErrProfileNotFound if loginProfileID doesn't exist or doesn't belong to userID (never reveals
// whether it belongs to someone else), or ErrLastLoginMethod if it's userID's only remaining
// LoginProfile.
func UnlinkIdentity(ctx context.Context, userID, loginProfileID uuid.UUID) error {
	profiles, err := loginProfilesForUser(ctx, userID)
	if err != nil {
		return err
	}

	found := false
	for _, p := range profiles {
		if p.ID == loginProfileID {
			found = true
			break
		}
	}
	if !found {
		return ErrProfileNotFound
	}
	if len(profiles) <= 1 {
		return ErrLastLoginMethod
	}

	// Hard delete, not soft: the (thirdPartyAuth, thirdPartyAuthId) unique index would otherwise
	// block relinking the same identity later against a soft-deleted row.
	filter := bson.D{{Key: "_id", Value: loginProfileID}, {Key: "userId", Value: userID}}
	result, err := database.Db.Collection(constants.LoginProfilesCollection).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrProfileNotFound
	}
	return nil
}

func loginProfilesForUser(ctx context.Context, userID uuid.UUID) ([]loginProfileDoc, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notDeletedFilter,
		bson.D{{Key: "userId", Value: userID}},
		bson.D{{Key: "thirdPartyAuth", Value: constants.Auth0ThirdPartyAuth}},
	}}}
	return database.Query[loginProfileDoc](constants.LoginProfilesCollection, filter, nil, nil, 0, 0)
}
