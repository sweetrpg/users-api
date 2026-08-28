package models

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFindProfileBySubject_ReturnsProvisionedFields(t *testing.T) {
	subject := "auth0|test-profile-find-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	provisioned, err := FindOrCreateUser(context.Background(), subject, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}

	profile, err := FindProfileBySubject(context.Background(), subject)
	if err != nil {
		t.Fatalf("FindProfileBySubject: %v", err)
	}
	if profile.UserID != provisioned.UserID {
		t.Errorf("UserID = %v, want %v", profile.UserID, provisioned.UserID)
	}
	if profile.Name != "Ada" || profile.Email != "ada@example.com" {
		t.Errorf("Name/Email = %q/%q, want Ada/ada@example.com", profile.Name, profile.Email)
	}
	if profile.Bio != "" || profile.Website != "" {
		t.Errorf("Bio/Website = %q/%q, want empty for a freshly provisioned user", profile.Bio, profile.Website)
	}
}

func TestFindProfileBySubject_UnknownSubjectReturnsNotFound(t *testing.T) {
	_, err := FindProfileBySubject(context.Background(), "auth0|never-provisioned-"+t.Name())
	if err != ErrProfileNotFound {
		t.Errorf("err = %v, want ErrProfileNotFound", err)
	}
}

func TestUpdateProfile_UpdatesNameBioWebsiteNotEmail(t *testing.T) {
	subject := "auth0|test-profile-update-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	provisioned, err := FindOrCreateUser(context.Background(), subject, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}

	if err := UpdateProfile(context.Background(), provisioned.UserID, "Ada Lovelace", "Mathematician", "https://example.com"); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	profile, err := FindProfileBySubject(context.Background(), subject)
	if err != nil {
		t.Fatalf("FindProfileBySubject: %v", err)
	}
	if profile.Name != "Ada Lovelace" || profile.Bio != "Mathematician" || profile.Website != "https://example.com" {
		t.Errorf("Name/Bio/Website = %q/%q/%q, want updated values", profile.Name, profile.Bio, profile.Website)
	}
	if profile.Email != "ada@example.com" {
		t.Errorf("Email = %q, want unchanged ada@example.com", profile.Email)
	}
}

func TestUpdateProfile_UnknownUserIDReturnsNotFound(t *testing.T) {
	err := UpdateProfile(context.Background(), uuid.New(), "x", "y", "")
	if err != ErrProfileNotFound {
		t.Errorf("err = %v, want ErrProfileNotFound", err)
	}
}

// TestFindProfileBySubject_ResolvesLegacyUserWithStringID regression-tests a real dev incident:
// FindOrCreateUser's legacy-string-_id adoption (see
// TestFindOrCreateUser_AdoptsLegacyUserWithStringID) links a new LoginProfile to that User's own
// id - but FindProfileBySubject's own User lookup used a bare uuid.UUID equality filter, which
// never matches a string-encoded _id. A caller could provision fine (created=false, found the
// existing LoginProfile) and then get a 404 on every subsequent profile fetch for the same
// subject, since the LoginProfile->User join couldn't find the User it just linked.
func TestFindProfileBySubject_ResolvesLegacyUserWithStringID(t *testing.T) {
	email := "legacy-string-id-profile-" + t.Name() + "@example.com"
	subject := "auth0|test-profile-string-id-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })
	t.Cleanup(func() { cleanupEmail(t, email) })

	legacyUserID := uuid.New()
	_, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(),
		bson.D{
			{Key: "_id", Value: legacyUserID.String()},
			{Key: "name", Value: "Legacy Admin"},
			{Key: "email", Value: email},
		})
	if err != nil {
		t.Fatalf("seeding legacy user: %v", err)
	}

	if _, err := FindOrCreateUser(context.Background(), subject, "Legacy Admin", email); err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}

	profile, err := FindProfileBySubject(context.Background(), subject)
	if err != nil {
		t.Fatalf("FindProfileBySubject: %v (legacy string _id should still resolve)", err)
	}
	if profile.UserID != legacyUserID {
		t.Errorf("UserID = %v, want %v", profile.UserID, legacyUserID)
	}
	if profile.Name != "Legacy Admin" || profile.Email != email {
		t.Errorf("Name/Email = %q/%q, want Legacy Admin/%s", profile.Name, profile.Email, email)
	}

	if err := UpdateProfile(context.Background(), legacyUserID, "Updated Name", "bio", "https://example.com"); err != nil {
		t.Fatalf("UpdateProfile against a legacy string _id: %v", err)
	}
}

// TestFindProfileBySubject_ResolvesLegacyUserWithUppercaseStringID regression-tests a real dev
// incident found immediately after the fix above shipped: the actual legacy document's `_id` is
// stored *uppercase* (matching Swift's `UUID.uuidString` convention, the Fluent-era service this
// data predates), not lowercase like the previous test's seed data or Go's own
// `uuid.UUID.String()`. MongoDB string comparison is case-sensitive, so the lowercase-only fix
// still 404'd for this specific, real account.
func TestFindProfileBySubject_ResolvesLegacyUserWithUppercaseStringID(t *testing.T) {
	email := "legacy-uppercase-id-profile-" + t.Name() + "@example.com"
	subject := "auth0|test-profile-uppercase-id-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })
	t.Cleanup(func() { cleanupEmail(t, email) })

	legacyUserID := uuid.New()
	_, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(),
		bson.D{
			{Key: "_id", Value: strings.ToUpper(legacyUserID.String())},
			{Key: "name", Value: "Legacy Admin"},
			{Key: "email", Value: email},
		})
	if err != nil {
		t.Fatalf("seeding legacy user: %v", err)
	}

	if _, err := FindOrCreateUser(context.Background(), subject, "Legacy Admin", email); err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}

	profile, err := FindProfileBySubject(context.Background(), subject)
	if err != nil {
		t.Fatalf("FindProfileBySubject: %v (uppercase legacy string _id should still resolve)", err)
	}
	if profile.UserID != legacyUserID {
		t.Errorf("UserID = %v, want %v", profile.UserID, legacyUserID)
	}

	if err := UpdateProfile(context.Background(), legacyUserID, "Updated Name", "bio", "https://example.com"); err != nil {
		t.Fatalf("UpdateProfile against an uppercase legacy string _id: %v", err)
	}
}
