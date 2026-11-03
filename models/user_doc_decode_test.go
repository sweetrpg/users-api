package models

import (
	"testing"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func decodeUserDoc(t *testing.T, id any) userDoc {
	t.Helper()
	raw, err := bson.Marshal(bson.D{{Key: "_id", Value: id}, {Key: "name", Value: "N"}, {Key: "email", Value: "e@x.test"}, {Key: "username", Value: "nn"}, {Key: "created_by", Value: "c"}})
	if err != nil {
		t.Fatal(err)
	}
	var d userDoc
	if err := bson.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decode %T id: %v", id, err)
	}
	return d
}

func TestUserDocDecodesEveryStoredIDForm(t *testing.T) {
	want := uuid.MustParse("b3384f5d-1111-4222-8333-444455556666")
	forms := map[string]any{
		"binary uuid":      want,
		"lowercase string": want.String(),
		"uppercase string": "B3384F5D-1111-4222-8333-444455556666",
	}
	for name, id := range forms {
		t.Run(name, func(t *testing.T) {
			d := decodeUserDoc(t, id)
			if d.ID != want || d.Name != "N" || d.Email != "e@x.test" || d.Username != "nn" || d.CreatedBy != "c" {
				t.Errorf("doc = %+v", d)
			}
		})
	}
}

func TestUserDocRejectsAnIDThatIsNotAUUID(t *testing.T) {
	raw, _ := bson.Marshal(bson.D{{Key: "_id", Value: "not-a-uuid"}})
	var d userDoc
	if err := bson.Unmarshal(raw, &d); err == nil {
		t.Error("expected an error for a non-UUID string _id")
	}
}

func TestUserDocStillEncodesTheIDAsBinary(t *testing.T) {
	id := uuid.New()
	got, err := bson.Marshal(userDoc{ID: id, Name: "N"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := bson.Marshal(bson.D{{Key: "_id", Value: id}})
	if gv, wv := bson.Raw(got).Lookup("_id"), bson.Raw(want).Lookup("_id"); gv.Type != wv.Type || string(gv.Value) != string(wv.Value) {
		t.Errorf("_id encoded as %v, want %v", gv, wv)
	}
}
