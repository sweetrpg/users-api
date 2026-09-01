package models

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrUserNotFound means no live user matched a lookup by id, email, or username.
var ErrUserNotFound = errors.New("models: user not found")

const (
	usernameMinLength = 3
	usernameMaxLength = 30
	// usernameCollisionTries bounds the -2, -3, ... suffixing GenerateUsername does before
	// giving up. 50 is far more than any real collision cluster.
	usernameCollisionTries = 50
)

// usernameFormat is the accepted shape for a user-chosen username: lowercase letters, digits,
// underscore, hyphen, 3-30 chars. Generated usernames also satisfy it.
var usernameFormat = regexp.MustCompile(`^[a-z0-9_-]{3,30}$`)

var usernameStripRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// ValidUsername reports whether s is an acceptable user-chosen username.
func ValidUsername(s string) bool {
	return usernameFormat.MatchString(s)
}

// usernameSeed picks the string GenerateUsername derives a username from: the email (its local
// part is used) when present, otherwise the display name.
func usernameSeed(email, name string) string {
	if strings.TrimSpace(email) != "" {
		return email
	}
	return name
}

// slugifyUsername lowercases seed and reduces it to the allowed character set, collapsing runs
// of disallowed characters to a single hyphen and trimming leading/trailing separators. A seed
// that reduces to fewer than usernameMinLength usable characters is padded from a fixed base so
// the result always satisfies usernameFormat.
func slugifyUsername(seed string) string {
	s := strings.ToLower(strings.TrimSpace(seed))
	// An email seed: keep only the local part.
	if at := strings.IndexByte(s, '@'); at >= 0 {
		s = s[:at]
	}
	s = usernameStripRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if len(s) < usernameMinLength {
		s = (s + "user")[:usernameMinLength]
	}
	if len(s) > usernameMaxLength {
		s = strings.Trim(s[:usernameMaxLength], "-_")
	}
	return s
}

// usernameTaken reports whether any live user already holds username.
func usernameTaken(ctx context.Context, username string) (bool, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "username", Value: username}},
	}}}
	n, err := database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, filter)
	return n > 0, err
}

// GenerateUsername returns a username derived from seed that no live user currently holds,
// appending "-2", "-3", ... on collision. Returns an error only if the DB is unreachable or the
// (very wide) collision bound is exhausted.
func GenerateUsername(ctx context.Context, seed string) (string, error) {
	base := slugifyUsername(seed)
	for i := 1; i <= usernameCollisionTries; i++ {
		candidate := base
		if i > 1 {
			suffix := fmt.Sprintf("-%d", i)
			candidate = base
			if len(candidate)+len(suffix) > usernameMaxLength {
				candidate = candidate[:usernameMaxLength-len(suffix)]
			}
			candidate += suffix
		}
		taken, err := usernameTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("models: no free username after %d tries for seed %q", usernameCollisionTries, seed)
}

// EnsureUsername assigns a generated username to userID if it doesn't have one yet. A no-op for
// a user that already has a username. Losing a race with a concurrent backfill (duplicate-key
// on the write) is treated as success - the other writer set it.
func EnsureUsername(ctx context.Context, userID uuid.UUID, seed string) error {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		userIDFilter(userID),
	}}}

	var doc userDoc
	err := database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrProfileNotFound
		}
		return err
	}
	if doc.Username != "" {
		return nil
	}

	username, err := GenerateUsername(ctx, seed)
	if err != nil {
		return err
	}

	setFilter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		userIDFilter(userID),
		bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "username", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "username", Value: ""}},
		}}},
	}}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "username", Value: username}}}}
	if _, err := database.Db.Collection(constants.UsersCollection).UpdateOne(ctx, setFilter, update); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}
		return err
	}
	return nil
}

// FindUserIDByUsername resolves a username to its live user's id, or ErrUserNotFound.
func FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "username", Value: username}},
	}}}

	var raw bson.Raw
	err := database.Db.Collection(constants.UsersCollection).FindOne(ctx, filter).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return uuid.UUID{}, ErrUserNotFound
		}
		return uuid.UUID{}, err
	}
	id, err := decodeFlexibleUUID(raw.Lookup("_id"))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("decoding user _id for username %q: %w", username, err)
	}
	return id, nil
}

// FindUserIDByEmail resolves an email to its live user's id, or ErrUserNotFound. Reuses
// findUserByEmail's lenient legacy-id decoding.
func FindUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	user, err := findUserByEmail(ctx, email)
	if err != nil {
		return uuid.UUID{}, err
	}
	if user == nil {
		return uuid.UUID{}, ErrUserNotFound
	}
	return user.ID, nil
}

// userExists reports whether id names a live user - used to reject a friend request aimed at a
// well-formed UUID that isn't anyone.
func userExists(ctx context.Context, id uuid.UUID) (bool, error) {
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		userIDFilter(id),
	}}}
	n, err := database.Db.Collection(constants.UsersCollection).CountDocuments(ctx, filter)
	return n > 0, err
}
