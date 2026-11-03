package models

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeIndexer struct {
	createErr error
	existing  []bson.Raw
	created   int
}

func (f *fakeIndexer) create(context.Context, mongo.IndexModel) error {
	f.created++
	return f.createErr
}
func (f *fakeIndexer) list(context.Context) ([]bson.Raw, error) { return f.existing, nil }

func spec(t *testing.T, d bson.D) bson.Raw {
	t.Helper()
	b, err := bson.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func conflict(code int32) error { return mongo.CommandError{Code: code, Name: "IndexOptionsConflict"} }

var emailModel = mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)}

func TestEnsureIndexAcceptsAnEquivalentIndexUnderAnotherName(t *testing.T) {
	ix := &fakeIndexer{createErr: conflict(85), existing: []bson.Raw{
		spec(t, bson.D{{Key: "name", Value: "_id_"}, {Key: "key", Value: bson.D{{Key: "_id", Value: 1}}}}),
		spec(t, bson.D{{Key: "name", Value: "unique"}, {Key: "key", Value: bson.D{{Key: "email", Value: int32(1)}}}, {Key: "unique", Value: true}}),
	}}
	if err := ensureIndex(context.Background(), ix, emailModel); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestEnsureIndexStillFailsOnARealConflict(t *testing.T) {
	notUnique := spec(t, bson.D{{Key: "name", Value: "unique"}, {Key: "key", Value: bson.D{{Key: "email", Value: 1}}}})
	ix := &fakeIndexer{createErr: conflict(86), existing: []bson.Raw{notUnique}}
	if err := ensureIndex(context.Background(), ix, emailModel); err == nil {
		t.Error("a non-unique email index must not satisfy a unique one")
	}
}

func TestEnsureIndexDoesNotListWhenCreateSucceedsOrFailsForOtherReasons(t *testing.T) {
	if err := ensureIndex(context.Background(), &fakeIndexer{}, emailModel); err != nil {
		t.Errorf("err = %v", err)
	}
	boom := errors.New("network")
	if err := ensureIndex(context.Background(), &fakeIndexer{createErr: boom}, emailModel); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the original", err)
	}
}

func TestEquivalenceChecksSparseAndExpiry(t *testing.T) {
	username := mongo.IndexModel{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)}
	key := bson.D{{Key: "username", Value: 1}}
	cases := []struct {
		name string
		have bson.D
		want mongo.IndexModel
		ok   bool
	}{
		{"sparse matches", bson.D{{Key: "key", Value: key}, {Key: "unique", Value: true}, {Key: "sparse", Value: true}}, username, true},
		{"missing sparse differs", bson.D{{Key: "key", Value: key}, {Key: "unique", Value: true}}, username, false},
		{"ttl matches", bson.D{{Key: "key", Value: bson.D{{Key: "createdAt", Value: 1}}}, {Key: "expireAfterSeconds", Value: int32(600)}},
			mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(600)}, true},
		{"ttl differs", bson.D{{Key: "key", Value: bson.D{{Key: "createdAt", Value: 1}}}, {Key: "expireAfterSeconds", Value: int32(60)}},
			mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(600)}, false},
		{"direction differs", bson.D{{Key: "key", Value: bson.D{{Key: "email", Value: -1}}}, {Key: "unique", Value: true}}, emailModel, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := equivalentIndex(spec(t, c.have), c.want); got != c.ok {
				t.Errorf("equivalent = %v, want %v", got, c.ok)
			}
		})
	}
}
