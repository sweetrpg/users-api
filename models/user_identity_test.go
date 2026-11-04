package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildIdentitiesListsEveryLinkedSubjectPerUser(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	identities := buildIdentities(
		[]*userDoc{{ID: a, Email: "a@x.test"}, {ID: b, Email: "b@x.test"}, {ID: c, Email: "c@x.test"}},
		[]*loginProfileDoc{
			{UserID: a, ThirdPartyAuthID: "google-oauth2|1"},
			{UserID: b, ThirdPartyAuthID: "github|9"},
			{UserID: a, ThirdPartyAuthID: "auth0|2"},
		},
	)
	if len(identities) != 3 {
		t.Fatalf("identities = %d, want one per user", len(identities))
	}
	if got := identities[0].Subjects; len(got) != 2 || got[0] != "google-oauth2|1" || got[1] != "auth0|2" {
		t.Errorf("a.Subjects = %v", got)
	}
	if identities[0].Subject == nil || *identities[0].Subject != "google-oauth2|1" {
		t.Errorf("a.Subject = %v, want the first subject", identities[0].Subject)
	}
	if identities[2].Subject != nil || identities[2].Subjects == nil || len(identities[2].Subjects) != 0 {
		t.Errorf("a user with no login: %+v", identities[2])
	}
}

func TestIdentityJSONHasSubjectsArrayEvenWhenEmpty(t *testing.T) {
	out, _ := json.Marshal(buildIdentities([]*userDoc{{ID: uuid.New(), Email: "x@x.test"}}, nil)[0])
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if s, ok := m["subjects"].([]any); !ok || len(s) != 0 {
		t.Errorf("json = %s, want subjects: []", out)
	}
	if _, has := m["subject"]; has {
		t.Errorf("json = %s, subject should be omitted", out)
	}
}
