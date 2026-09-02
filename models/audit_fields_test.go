package models

import (
	"context"
	"testing"

	modelcore "github.com/sweetrpg/model-core.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// These assert the platform audit-fields convention (PADR-0001) on users-api's write paths.
// DB-backed; TestMain skips the whole package without DB_URI.

func TestFindOrCreateUser_StampsAuditFieldsOnUserAndLoginProfile(t *testing.T) {
	ctx := context.Background()
	subject := "auth0|test-audit-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	result, err := FindOrCreateUser(ctx, subject, "Ada", "ada-audit@example.com")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}
	t.Cleanup(func() { cleanupEmail(t, "ada-audit@example.com") })

	var user userDoc
	if err := database.Db.Collection(constants.UsersCollection).
		FindOne(ctx, userIDFilter(result.UserID)).Decode(&user); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		t.Errorf("user created_at/updated_at are zero: %+v", user.Auditable)
	}
	if !user.CreatedAt.Equal(user.UpdatedAt) {
		t.Errorf("fresh user created_at %v != updated_at %v", user.CreatedAt, user.UpdatedAt)
	}
	want := result.UserID.String()
	if user.CreatedBy != want || user.UpdatedBy != want {
		t.Errorf("user *_by = (%q, %q), want (%q, %q) - the self-provisioning user", user.CreatedBy, user.UpdatedBy, want, want)
	}
	if user.DeletedAt != nil || user.DeletedBy != nil {
		t.Errorf("fresh user has non-nil deleted_at/deleted_by: %+v", user.Auditable)
	}

	var profile loginProfileDoc
	if err := database.Db.Collection(constants.LoginProfilesCollection).
		FindOne(ctx, bson.D{{Key: "thirdPartyAuthId", Value: subject}}).Decode(&profile); err != nil {
		t.Fatalf("load login_profile: %v", err)
	}
	if profile.CreatedAt.IsZero() || profile.CreatedBy != want {
		t.Errorf("login_profile audit fields = %+v, want created_by %q and a real created_at", profile.Auditable, want)
	}
}

func TestSendFriendRequest_StampsCreateAuditToCaller(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if fr.CreatedAt.IsZero() || !fr.CreatedAt.Equal(fr.UpdatedAt) {
		t.Errorf("fresh friendship created_at/updated_at = (%v, %v), want equal and non-zero", fr.CreatedAt, fr.UpdatedAt)
	}
	if fr.CreatedBy != a.String() || fr.UpdatedBy != a.String() {
		t.Errorf("friendship *_by = (%q, %q), want the caller %q", fr.CreatedBy, fr.UpdatedBy, a.String())
	}
}

func TestAcceptFriendRequest_AdvancesUpdateAuditToAccepter(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if err := AcceptFriendRequest(ctx, b, fr.ID); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}

	var got friendshipDoc
	if err := database.Db.Collection(constants.FriendshipsCollection).
		FindOne(ctx, bson.D{{Key: "_id", Value: fr.ID}}).Decode(&got); err != nil {
		t.Fatalf("reload friendship: %v", err)
	}
	if got.UpdatedBy != b.String() {
		t.Errorf("updated_by = %q after accept, want the accepter %q", got.UpdatedBy, b.String())
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Errorf("updated_at %v not after created_at %v", got.UpdatedAt, got.CreatedAt)
	}
	if got.CreatedBy != a.String() {
		t.Errorf("created_by = %q after accept, want the original requester %q (unchanged)", got.CreatedBy, a.String())
	}
}

func TestSystemActorConstantIsWired(t *testing.T) {
	if modelcore.SystemActor != "system" {
		t.Fatalf("modelcore.SystemActor = %q, want \"system\"", modelcore.SystemActor)
	}
}
