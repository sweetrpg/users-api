package models

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func cleanupUsername(t *testing.T, username string) {
	t.Helper()
	_, _ = database.Db.Collection(constants.UsersCollection).
		DeleteMany(context.Background(), bson.D{{Key: "username", Value: username}})
}

// seedUser inserts a bare user doc and registers cleanup by id + username.
func seedUser(t *testing.T, name, email, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	doc := bson.D{{Key: "_id", Value: id}, {Key: "name", Value: name}, {Key: "email", Value: email}}
	if username != "" {
		doc = append(doc, bson.E{Key: "username", Value: username})
	}
	if _, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(), doc); err != nil {
		t.Fatalf("seedUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.UsersCollection).
			DeleteMany(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
	return id
}

func TestValidUsername(t *testing.T) {
	ok := []string{"ada", "ada-lovelace", "user_1", "abc", strings.Repeat("a", 30)}
	bad := []string{"", "ab", "Ada", "has space", "no.dots", "emoji-\U0001F600", strings.Repeat("a", 31)}
	for _, s := range ok {
		if !ValidUsername(s) {
			t.Errorf("ValidUsername(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if ValidUsername(s) {
			t.Errorf("ValidUsername(%q) = true, want false", s)
		}
	}
}

func TestSlugifyUsername(t *testing.T) {
	cases := map[string]string{
		"ada@example.com":   "ada",
		"Ada Lovelace":      "ada-lovelace",
		"  weird__name!!  ": "weird__name",
		"x":                 "xus", // padded from "x" + "user"
		"a.b.c@example.com": "a-b-c",
	}
	for in, want := range cases {
		if got := slugifyUsername(in); got != want {
			t.Errorf("slugifyUsername(%q) = %q, want %q", in, got, want)
		}
	}
	if got := slugifyUsername("Ada Lovelace"); !ValidUsername(got) {
		t.Errorf("slugifyUsername output %q is not a valid username", got)
	}
}

func TestGenerateUsername_FreeBaseAndCollisionSuffix(t *testing.T) {
	ctx := context.Background()
	base := "gtest-" + strings.ToLower(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupUsername(t, base); cleanupUsername(t, base+"-2") })

	got, err := GenerateUsername(ctx, base+"@example.com")
	if err != nil {
		t.Fatalf("GenerateUsername: %v", err)
	}
	if got != base {
		t.Fatalf("GenerateUsername = %q, want the free base %q", got, base)
	}

	seedUser(t, "x", base+"-taken@example.com", base)
	got2, err := GenerateUsername(ctx, base+"@example.com")
	if err != nil {
		t.Fatalf("GenerateUsername after collision: %v", err)
	}
	if got2 != base+"-2" {
		t.Errorf("GenerateUsername after collision = %q, want %q", got2, base+"-2")
	}
}

func TestEnsureUsername_SetsWhenMissingNoopWhenPresent(t *testing.T) {
	ctx := context.Background()
	id := seedUser(t, "Grace Hopper", "grace-"+t.Name()+"@example.com", "")

	if err := EnsureUsername(ctx, id, "grace-"+t.Name()+"@example.com"); err != nil {
		t.Fatalf("EnsureUsername (set): %v", err)
	}
	var doc userDoc
	if err := database.Db.Collection(constants.UsersCollection).
		FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if doc.Username == "" {
		t.Fatal("EnsureUsername did not set a username")
	}
	first := doc.Username

	if err := EnsureUsername(ctx, id, "totally-different-seed"); err != nil {
		t.Fatalf("EnsureUsername (noop): %v", err)
	}
	_ = database.Db.Collection(constants.UsersCollection).
		FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if doc.Username != first {
		t.Errorf("EnsureUsername overwrote an existing username: %q -> %q", first, doc.Username)
	}
}

func TestFindUserIDByUsernameAndEmail(t *testing.T) {
	ctx := context.Background()
	username := "lookup-" + strings.ToLower(uuid.NewString()[:8])
	email := username + "@example.com"
	id := seedUser(t, "Lookup Target", email, username)

	if got, err := FindUserIDByUsername(ctx, username); err != nil || got != id {
		t.Errorf("FindUserIDByUsername = %v, %v; want %v, nil", got, err, id)
	}
	if _, err := FindUserIDByUsername(ctx, "no-such-"+username); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("FindUserIDByUsername miss err = %v, want ErrUserNotFound", err)
	}
	if got, err := FindUserIDByEmail(ctx, email); err != nil || got != id {
		t.Errorf("FindUserIDByEmail = %v, %v; want %v, nil", got, err, id)
	}
	if _, err := FindUserIDByEmail(ctx, "no-such-"+email); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("FindUserIDByEmail miss err = %v, want ErrUserNotFound", err)
	}
}
