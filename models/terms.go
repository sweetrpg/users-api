package models

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TermsAccepted is the version of the terms of service a user accepted and when. It is set only
// by the user for themselves, and replaced only by accepting a newer version.
type TermsAccepted struct {
	Version    string    `json:"version"`
	AcceptedAt time.Time `json:"accepted_at"`
}

// TermsNotNewerError means the version offered is older than the one already accepted. Current is
// the stored version.
type TermsNotNewerError struct{ Current string }

func (e *TermsNotNewerError) Error() string {
	return fmt.Sprintf("models: terms version is not newer than the accepted %s", e.Current)
}

// ErrTermsContention means the write lost a race with another acceptance on the same profile
// more times than a retry covers.
var ErrTermsContention = errors.New("models: terms acceptance kept changing under us")

// versionPattern is dotted numeric, one to four segments, each without leading zeros: 1.0, 1.0.1.
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})(\.(0|[1-9][0-9]{0,5})){0,3}$`)

// ValidTermsVersion reports whether v is a dotted numeric version such as 1.0 or 1.0.1.
func ValidTermsVersion(v string) bool { return versionPattern.MatchString(v) }

// CompareTermsVersions compares two valid versions segment by segment as numbers, treating a
// missing segment as 0, so 1.2 < 1.10 and 1.0 == 1.0.0. It returns -1, 0, or 1.
func CompareTermsVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(as), len(bs)); i++ {
		av, bv := segment(as, i), segment(bs, i)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}

func segment(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}

// decideTerms applies the acceptance rule to the stored record: the first acceptance and a newer
// version are written; the same version (however spelled) returns the stored record untouched, so
// accepted_at stays the original; an older version is refused.
func decideTerms(current *TermsAccepted, version string, now time.Time) (next TermsAccepted, write bool, err error) {
	if current == nil {
		return TermsAccepted{Version: version, AcceptedAt: now}, true, nil
	}
	switch CompareTermsVersions(version, current.Version) {
	case 1:
		return TermsAccepted{Version: version, AcceptedAt: now}, true, nil
	case 0:
		return *current, false, nil
	}
	return *current, false, &TermsNotNewerError{Current: current.Version}
}

// mongoTime is a time as MongoDB keeps it: UTC, to the millisecond.
func mongoTime(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// TermsStore records a user's acceptance of the terms. Accept takes the owner's own user ID; there
// is no overload that targets a user by any other ID, and the handler takes the ID from the
// verified token.
type TermsStore interface {
	Accept(ctx context.Context, ownerUserID uuid.UUID, version string, now time.Time) (TermsAccepted, error)
}

// MongoTermsStore is the TermsStore over the users collection.
type MongoTermsStore struct{}

// termsAttempts bounds the compare-and-set retries when two acceptances race.
const termsAttempts = 3

// Accept reads the stored record, applies decideTerms, and writes with a guard on the version it
// read, so a concurrent acceptance makes the write match nothing and the loop re-reads instead of
// overwriting a newer version with an older one. Returns ErrProfileNotFound if the user does not
// exist or is soft-deleted.
func (MongoTermsStore) Accept(ctx context.Context, owner uuid.UUID, version string, now time.Time) (TermsAccepted, error) {
	now = mongoTime(now)
	users := database.Db.Collection(constants.UsersCollection)
	for range termsAttempts {
		var raw bson.Raw
		err := users.FindOne(ctx, termsOwnerFilter(owner)).Decode(&raw)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return TermsAccepted{}, ErrProfileNotFound
		}
		if err != nil {
			return TermsAccepted{}, err
		}
		current := decodeTermsAccepted(raw)
		next, write, err := decideTerms(current, version, now)
		if err != nil || !write {
			return next, err
		}
		res, err := users.UpdateOne(ctx, termsWriteFilter(owner, current), termsUpdate(owner, next))
		if err != nil {
			return TermsAccepted{}, err
		}
		if res.MatchedCount == 1 {
			return next, nil
		}
	}
	return TermsAccepted{}, ErrTermsContention
}

// termsOwnerFilter matches the owner's live user document.
func termsOwnerFilter(owner uuid.UUID) bson.D {
	return bson.D{{Key: "$and", Value: bson.A{notDeletedFilter, userIDFilter(owner)}}}
}

// termsWriteFilter is the owner filter plus a guard that the stored acceptance is still the one
// that was read (or still absent), which makes the update a compare-and-set.
func termsWriteFilter(owner uuid.UUID, current *TermsAccepted) bson.D {
	guard := bson.D{{Key: "terms_accepted", Value: bson.D{{Key: "$exists", Value: false}}}}
	if current != nil {
		guard = bson.D{{Key: "terms_accepted.version", Value: current.Version}}
	}
	return bson.D{{Key: "$and", Value: bson.A{notDeletedFilter, userIDFilter(owner), guard}}}
}

// termsUpdate sets the acceptance and stamps the audit fields; the owner is the acting user.
func termsUpdate(owner uuid.UUID, next TermsAccepted) bson.D {
	return bson.D{{Key: "$set", Value: bson.D{
		{Key: "terms_accepted", Value: bson.D{
			{Key: "version", Value: next.Version},
			{Key: "accepted_at", Value: next.AcceptedAt},
		}},
		{Key: "updated_at", Value: next.AcceptedAt},
		{Key: "updated_by", Value: owner.String()},
	}}}
}

// decodeTermsAccepted reads terms_accepted from a user document, nil when absent or malformed.
func decodeTermsAccepted(raw bson.Raw) *TermsAccepted {
	doc, ok := raw.Lookup("terms_accepted").DocumentOK()
	if !ok {
		return nil
	}
	version, ok := doc.Lookup("version").StringValueOK()
	if !ok || version == "" {
		return nil
	}
	at, _ := doc.Lookup("accepted_at").TimeOK()
	return &TermsAccepted{Version: version, AcceptedAt: at.UTC()}
}

// MemoryTermsStore is an in-memory TermsStore with the same rules and date precision as the
// MongoDB one, for tests that must not need a database.
type MemoryTermsStore struct {
	mu    sync.Mutex
	terms map[uuid.UUID]*TermsAccepted
	users map[uuid.UUID]bool
}

// NewMemoryTermsStore returns a store that knows the given users.
func NewMemoryTermsStore(users ...uuid.UUID) *MemoryTermsStore {
	m := &MemoryTermsStore{terms: map[uuid.UUID]*TermsAccepted{}, users: map[uuid.UUID]bool{}}
	for _, u := range users {
		m.users[u] = true
	}
	return m
}

// Get returns a user's stored acceptance, nil if none.
func (m *MemoryTermsStore) Get(owner uuid.UUID) (*TermsAccepted, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.terms[owner]; t != nil {
		c := *t
		return &c, m.users[owner]
	}
	return nil, m.users[owner]
}

// Accept applies the acceptance rule to the owner's record.
func (m *MemoryTermsStore) Accept(_ context.Context, owner uuid.UUID, version string, now time.Time) (TermsAccepted, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.users[owner] {
		return TermsAccepted{}, ErrProfileNotFound
	}
	next, write, err := decideTerms(m.terms[owner], version, mongoTime(now))
	if err != nil || !write {
		return next, err
	}
	m.terms[owner] = &next
	return next, nil
}
