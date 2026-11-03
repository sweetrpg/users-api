package models

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func TestValidTermsVersion(t *testing.T) {
	valid := []string{"1.0", "1.0.1", "2", "10.20.30", "0.1", "1.2.3.4", "999999.0"}
	invalid := []string{"", "v1.0", "1.", ".1", "1..0", "1.0-beta", "01.0", "1.00", "a.b", "1.0.0.0.0", "1234567.0", " 1.0", "1.0 ", "1,0", "-1.0"}
	for _, v := range valid {
		if !ValidTermsVersion(v) {
			t.Errorf("%q rejected, want valid", v)
		}
	}
	for _, v := range invalid {
		if ValidTermsVersion(v) {
			t.Errorf("%q accepted, want invalid", v)
		}
	}
}

func TestCompareTermsVersionsIsNumericPerSegment(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "1.0.0", 0},
		{"1.2", "1.10", -1},
		{"1.10", "1.2", 1},
		{"2.0", "1.9.9", 1},
		{"1.0.1", "1.0", 1},
		{"1.0", "1.0.1", -1},
		{"9", "10", -1},
	}
	for _, c := range cases {
		if got := CompareTermsVersions(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

var t0 = time.Date(2026, 10, 31, 12, 0, 0, 123456789, time.UTC)

func TestDecideTerms(t *testing.T) {
	first := &TermsAccepted{Version: "1.1", AcceptedAt: t0}

	next, write, err := decideTerms(nil, "1.0", t0)
	if err != nil || !write || next.Version != "1.0" {
		t.Errorf("first acceptance: %+v write=%v err=%v", next, write, err)
	}
	next, write, err = decideTerms(first, "1.2", t0.Add(time.Hour))
	if err != nil || !write || next.Version != "1.2" || !next.AcceptedAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("newer: %+v write=%v err=%v", next, write, err)
	}
	next, write, err = decideTerms(first, "1.1", t0.Add(time.Hour))
	if err != nil || write || next != *first {
		t.Errorf("same version must return the stored record untouched: %+v write=%v err=%v", next, write, err)
	}
	next, write, err = decideTerms(first, "1.1.0", t0.Add(time.Hour))
	if err != nil || write || next != *first {
		t.Errorf("an equal version spelled differently is the same version: %+v write=%v err=%v", next, write, err)
	}
	var notNewer *TermsNotNewerError
	if _, write, err = decideTerms(first, "1.0", t0); !errors.As(err, &notNewer) || write || notNewer.Current != "1.1" {
		t.Errorf("older: write=%v err=%v", write, err)
	}
}

func TestMemoryTermsStoreMirrorsMongoSemantics(t *testing.T) {
	ctx := context.Background()
	owner, other := uuid.New(), uuid.New()
	st := NewMemoryTermsStore(owner, other)

	got, err := st.Accept(ctx, owner, "1.0", t0)
	if err != nil {
		t.Fatal(err)
	}
	// MongoDB stores dates to the millisecond; the returned value must equal what a later read sees.
	if got.AcceptedAt.Nanosecond()%1_000_000 != 0 || !got.AcceptedAt.Equal(t0.Truncate(time.Millisecond)) {
		t.Errorf("accepted_at = %v, want %v truncated to the millisecond", got.AcceptedAt, t0.Truncate(time.Millisecond))
	}
	if got.AcceptedAt.Location() != time.UTC {
		t.Errorf("accepted_at is not UTC: %v", got.AcceptedAt.Location())
	}

	again, err := st.Accept(ctx, owner, "1.0", t0.Add(time.Hour))
	if err != nil || !again.AcceptedAt.Equal(got.AcceptedAt) {
		t.Errorf("repeat acceptance changed accepted_at: %+v err=%v", again, err)
	}
	if _, err := st.Accept(ctx, owner, "0.9", t0); err == nil {
		t.Error("an older version was accepted")
	}
	if cur, _ := st.Get(other); cur != nil {
		t.Errorf("another user's record changed: %+v", cur)
	}
	if _, err := st.Accept(ctx, uuid.New(), "1.0", t0); !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("unknown user: err = %v, want ErrProfileNotFound", err)
	}
}

// walk visits every value in a BSON document.
func walk(v any, visit func(key string, val any)) {
	switch x := v.(type) {
	case bson.D:
		for _, e := range x {
			visit(e.Key, e.Value)
			walk(e.Value, visit)
		}
	case bson.A:
		for _, item := range x {
			walk(item, visit)
		}
	}
}

func TestTermsMongoQueriesAreShapedLikeMongoNeedsThem(t *testing.T) {
	owner := uuid.New()
	for name, cur := range map[string]*TermsAccepted{"first acceptance": nil, "replacing": {Version: "1.0", AcceptedAt: t0}} {
		t.Run(name, func(t *testing.T) {
			filter := termsWriteFilter(owner, cur)
			walk(filter, func(key string, val any) {
				if key == "$in" {
					if arr, ok := val.(bson.A); !ok || len(arr) == 0 {
						t.Errorf("$in needs a non-empty array, got %#v", val)
					}
				}
			})
			if _, err := bson.Marshal(filter); err != nil {
				t.Errorf("filter does not marshal: %v", err)
			}
			update := termsUpdate(owner, TermsAccepted{Version: "1.1", AcceptedAt: t0})
			set, ok := update[0].Value.(bson.D)
			if update[0].Key != "$set" || !ok || len(set) == 0 {
				t.Fatalf("update = %#v, want a non-empty $set", update)
			}
			keys := map[string]bool{}
			for _, e := range set {
				keys[e.Key] = true
			}
			for _, want := range []string{"terms_accepted", "updated_at", "updated_by"} {
				if !keys[want] {
					t.Errorf("$set is missing %s (audit fields)", want)
				}
			}
		})
	}
}

func TestTermsWriteFilterIsScopedToTheOwnerAndTheStoredVersion(t *testing.T) {
	owner := uuid.New()
	hasIdIn, hasVersion, hasMissing := false, false, false
	walk(termsWriteFilter(owner, &TermsAccepted{Version: "1.0"}), func(key string, val any) {
		switch key {
		case "_id":
			hasIdIn = true
		case "terms_accepted.version":
			hasVersion = val == "1.0"
		}
	})
	walk(termsWriteFilter(owner, nil), func(key string, val any) {
		if key == "terms_accepted" {
			hasMissing = true
		}
	})
	if !hasIdIn || !hasVersion || !hasMissing {
		t.Errorf("owner scope %v, version guard %v, missing-record guard %v", hasIdIn, hasVersion, hasMissing)
	}
}

func TestDecodeTermsAccepted(t *testing.T) {
	mk := func(v any) bson.Raw {
		b, _ := bson.Marshal(bson.D{{Key: "terms_accepted", Value: v}})
		return b
	}
	got := decodeTermsAccepted(mk(bson.D{{Key: "version", Value: "1.0"}, {Key: "accepted_at", Value: t0.Truncate(time.Millisecond)}}))
	if got == nil || got.Version != "1.0" || !got.AcceptedAt.Equal(t0.Truncate(time.Millisecond)) || got.AcceptedAt.Location() != time.UTC {
		t.Errorf("decoded = %+v", got)
	}
	for name, raw := range map[string]bson.Raw{
		"absent":        mustMarshal(bson.D{{Key: "name", Value: "x"}}),
		"not a doc":     mk("1.0"),
		"no version":    mk(bson.D{{Key: "accepted_at", Value: t0}}),
		"empty version": mk(bson.D{{Key: "version", Value: ""}}),
	} {
		if d := decodeTermsAccepted(raw); d != nil {
			t.Errorf("%s: decoded %+v, want nil", name, d)
		}
	}
}

func mustMarshal(v any) bson.Raw {
	b, err := bson.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
