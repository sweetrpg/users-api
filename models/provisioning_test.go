package models

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// TestMain connects to the DB_URI-configured MongoDB (started as a CI service container, see
// .github/workflows/ci.yaml/pr.yaml) and ensures the unique index FindOrCreateUser relies on.
// Skips DB-backed tests entirely when DB_URI isn't set, so local `go test ./...` still runs
// without a database - matching this platform's "no external service dependencies" convention
// for local development; CI supplies DB_URI to get real coverage of the find-or-create and
// duplicate-key-race paths.
func TestMain(m *testing.M) {
	if os.Getenv("DB_URI") == "" {
		fmt.Println("DB_URI not set, skipping models package DB-backed tests")
		os.Exit(0)
	}

	logging.Init()
	database.SetupDatabase()
	if err := EnsureLoginProfileIndexes(context.Background()); err != nil {
		fmt.Println("failed to ensure login_profiles indexes:", err)
		os.Exit(1)
	}
	if err := EnsureUserIndexes(context.Background()); err != nil {
		fmt.Println("failed to ensure users indexes:", err)
		os.Exit(1)
	}

	code := m.Run()

	database.TeardownDatabase()
	os.Exit(code)
}

func cleanupSubject(t *testing.T, subject string) {
	t.Helper()
	ctx := context.Background()
	filter := bson.D{{Key: "thirdPartyAuthId", Value: subject}}
	_, _ = database.Db.Collection(constants.LoginProfilesCollection).DeleteMany(ctx, filter)
}

func cleanupEmail(t *testing.T, email string) {
	t.Helper()
	ctx := context.Background()
	filter := bson.D{{Key: "email", Value: email}}
	_, _ = database.Db.Collection(constants.UsersCollection).DeleteMany(ctx, filter)
}

func TestFindOrCreateUser_NewSubjectCreatesUserAndLoginProfile(t *testing.T) {
	subject := "auth0|test-new-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	result, err := FindOrCreateUser(context.Background(), subject, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}
	if !result.Created {
		t.Errorf("Created = false, want true for a new subject")
	}
	if result.UserID.String() == "" {
		t.Errorf("UserID is empty")
	}
}

func TestFindOrCreateUser_RepeatCallReturnsSameUserID(t *testing.T) {
	subject := "auth0|test-repeat-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	first, err := FindOrCreateUser(context.Background(), subject, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("first FindOrCreateUser: %v", err)
	}

	second, err := FindOrCreateUser(context.Background(), subject, "ignored", "ignored@example.com")
	if err != nil {
		t.Fatalf("second FindOrCreateUser: %v", err)
	}
	if second.Created {
		t.Errorf("Created = true on repeat call, want false")
	}
	if second.UserID != first.UserID {
		t.Errorf("UserID = %v on repeat call, want %v", second.UserID, first.UserID)
	}
}

func TestFindOrCreateUser_ConcurrentFirstLoginsDoNotDuplicate(t *testing.T) {
	subject := "auth0|test-concurrent-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	const callers = 10
	var wg sync.WaitGroup
	results := make([]ProvisionResult, callers)
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = FindOrCreateUser(context.Background(), subject, "Ada", "ada@example.com")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: FindOrCreateUser: %v", i, err)
		}
	}

	created := 0
	for i, r := range results {
		if r.UserID != results[0].UserID {
			t.Errorf("caller %d: UserID = %v, want %v (same as caller 0)", i, r.UserID, results[0].UserID)
		}
		if r.Created {
			created++
		}
	}
	if created != 1 {
		t.Errorf("created count = %d, want exactly 1 across %d concurrent callers", created, callers)
	}

	count, err := database.Db.Collection(constants.LoginProfilesCollection).CountDocuments(context.Background(),
		bson.D{{Key: "thirdPartyAuthId", Value: subject}})
	if err != nil {
		t.Fatalf("CountDocuments: %v", err)
	}
	if count != 1 {
		t.Errorf("login_profiles documents for subject = %d, want 1", count)
	}
}

// TestFindOrCreateUser_AdoptsExistingUserWithSameEmail regression-tests a real dev incident: a
// User document that predates the Auth0 login flow (no LoginProfile, e.g. a manually
// bootstrapped admin record) collided with FindOrCreateUser's blind insert on the unique email
// index, 500ing every first login for that email. The fix links a new LoginProfile to the
// existing User instead of erroring on the collision.
func TestFindOrCreateUser_AdoptsExistingUserWithSameEmail(t *testing.T) {
	email := "legacy-" + t.Name() + "@example.com"
	subject := "auth0|test-adopt-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })
	t.Cleanup(func() { cleanupEmail(t, email) })

	legacyUserID := uuid.New()
	_, err := database.Db.Collection(constants.UsersCollection).InsertOne(context.Background(),
		userDoc{ID: legacyUserID, Name: "Legacy Admin", Email: email})
	if err != nil {
		t.Fatalf("seeding legacy user: %v", err)
	}

	result, err := FindOrCreateUser(context.Background(), subject, "New Name", email)
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}
	if !result.Created {
		t.Errorf("Created = false, want true - this subject is new, even though it adopted an existing User")
	}
	if result.UserID != legacyUserID {
		t.Errorf("UserID = %v, want %v (the legacy User's own id)", result.UserID, legacyUserID)
	}

	count, err := database.Db.Collection(constants.UsersCollection).CountDocuments(context.Background(),
		bson.D{{Key: "email", Value: email}})
	if err != nil {
		t.Fatalf("CountDocuments: %v", err)
	}
	if count != 1 {
		t.Errorf("users documents for email = %d, want 1 (no duplicate created)", count)
	}
}
