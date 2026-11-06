package models

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func TestIsPlaceholderName(t *testing.T) {
	for name, want := range map[string]bool{"": true, "  ": true, "User": true, "user": true, " USER ": true, "shazean": false, "Ada Lovelace": false, "Userland": false} {
		if got := isPlaceholderName(name); got != want {
			t.Errorf("isPlaceholderName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestBuildIdentitiesCarriesNameAndUsernameAndOmitsEmptyOnes(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ids := buildIdentities([]*userDoc{{ID: a, Name: "shazean", Username: "shazean"}, {ID: b}}, nil)
	if ids[0].Name != "shazean" || ids[0].Username != "shazean" {
		t.Errorf("identity = %+v", ids[0])
	}
	if ids[1].Name != "" || ids[1].Username != "" {
		t.Errorf("unset fields should stay empty: %+v", ids[1])
	}
}

func insertNamedUser(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	doc := userDoc{ID: id, Name: name, Email: "refresh-" + id.String() + "@example.test"}
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
	return id
}

func storedName(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var d userDoc
	if err := database.Db.Collection(constants.UsersCollection).FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		t.Fatal(err)
	}
	return d.Name
}

func TestRefreshPlaceholderNameReplacesOnlyPlaceholders(t *testing.T) {
	ctx := context.Background()
	for _, stored := range []string{"User", "", "user"} {
		id := insertNamedUser(t, stored)
		if err := RefreshPlaceholderName(ctx, id, "shazean"); err != nil {
			t.Fatal(err)
		}
		if got := storedName(t, id); got != "shazean" {
			t.Errorf("stored %q: name = %q, want shazean", stored, got)
		}
	}
	real := insertNamedUser(t, "Ada Lovelace")
	if err := RefreshPlaceholderName(ctx, real, "shazean"); err != nil || storedName(t, real) != "Ada Lovelace" {
		t.Errorf("a real stored name must not be overwritten: %q, err %v", storedName(t, real), err)
	}
	ph := insertNamedUser(t, "User")
	if err := RefreshPlaceholderName(ctx, ph, "User"); err != nil || storedName(t, ph) != "User" {
		t.Errorf("a placeholder incoming name changes nothing: %q, err %v", storedName(t, ph), err)
	}
}

func TestFindOrCreateUserRefreshesAPlaceholderNameOnLogin(t *testing.T) {
	subject := "github|refresh-" + uuid.NewString()
	t.Cleanup(func() { cleanupSubject(t, subject) })
	email := "refresh-login-" + uuid.NewString() + "@example.test"
	t.Cleanup(func() { cleanupEmail(t, email) })

	first, err := FindOrCreateUser(context.Background(), subject, "User", email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FindOrCreateUser(context.Background(), subject, "shazean", email); err != nil {
		t.Fatal(err)
	}
	if got := storedName(t, first.UserID); got != "shazean" {
		t.Errorf("name after the second login = %q, want shazean", got)
	}
}
