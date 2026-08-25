package models

import (
	"context"
	"testing"

	"github.com/google/uuid"
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
