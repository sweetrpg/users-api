package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

var (
	userA = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userB = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

// termsFixture wires the profile routes to an in-memory terms store and a profile lookup, with
// auth-api stubbed to verify token "tok-a" as user A's subject and "tok-b" as user B's.
type termsFixture struct {
	t       *testing.T
	r       *gin.Engine
	store   *models.MemoryTermsStore
	subject map[string]uuid.UUID
}

func newTermsFixture(t *testing.T) *termsFixture {
	t.Helper()
	logging.Init()
	gin.SetMode(gin.TestMode)
	f := &termsFixture{t: t, store: models.NewMemoryTermsStore(userA, userB), subject: map[string]uuid.UUID{"auth0|a": userA, "auth0|b": userB}}

	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sub string
		switch strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") {
		case "tok-a":
			sub = "auth0|a"
		case "tok-b":
			sub = "auth0|b"
		case "tok-nobody":
			sub = "auth0|nobody"
		default:
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleUser}, Sub: sub})
	}))
	t.Cleanup(idp.Close)

	oldLookup, oldStore := profileLookup, termsStore
	profileLookup = func(_ context.Context, subject string) (*models.Profile, error) {
		id, ok := f.subject[subject]
		if !ok {
			return nil, models.ErrProfileNotFound
		}
		cur, _ := f.store.Get(id)
		return &models.Profile{UserID: id, Name: "Name", Email: "x@example.test", TermsAccepted: cur}, nil
	}
	termsStore = f.store
	t.Cleanup(func() { profileLookup, termsStore = oldLookup, oldStore })

	f.r = gin.New()
	setupProfileHandlers(f.r, authz.NewClient(idp.URL))
	return f
}

func (f *termsFixture) do(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func (f *termsFixture) accept(token, version string) *httptest.ResponseRecorder {
	return f.do(http.MethodPut, "/profile/terms", token, `{"version":"`+version+`"}`)
}

func code(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, want, rec.Body)
	}
}

func decodeAccepted(t *testing.T, rec *httptest.ResponseRecorder) models.TermsAccepted {
	t.Helper()
	var out models.TermsAccepted
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	return out
}

func TestAcceptTermsRequiresAVerifiedToken(t *testing.T) {
	f := newTermsFixture(t)
	code(t, f.accept("", "1.0"), http.StatusUnauthorized)
	code(t, f.accept("garbage", "1.0"), http.StatusUnauthorized)
}

func TestAcceptTermsRecordsVersionAndServerTimeInUTC(t *testing.T) {
	f := newTermsFixture(t)
	before := time.Now().UTC().Add(-time.Second)
	rec := f.accept("tok-a", "1.0")
	code(t, rec, http.StatusOK)
	got := decodeAccepted(t, rec)
	if got.Version != "1.0" || got.AcceptedAt.Before(before) || got.AcceptedAt.After(time.Now().UTC().Add(time.Second)) || got.AcceptedAt.Location() != time.UTC {
		t.Errorf("accepted = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"accepted_at"`) {
		t.Errorf("body = %s", rec.Body)
	}
	if stored, _ := f.store.Get(userA); stored == nil || stored.Version != "1.0" {
		t.Errorf("stored = %+v", stored)
	}
}

func TestAClientCannotChooseWhoAccepts(t *testing.T) {
	f := newTermsFixture(t)
	for _, body := range []string{
		`{"version":"1.0","user_id":"` + userB.String() + `"}`,
		`{"version":"1.0","userId":"` + userB.String() + `","id":"` + userB.String() + `","accepted_at":"2020-01-01T00:00:00Z"}`,
	} {
		code(t, f.do(http.MethodPut, "/profile/terms", "tok-a", body), http.StatusOK)
	}
	if b, _ := f.store.Get(userB); b != nil {
		t.Errorf("user B's record was written by user A's request: %+v", b)
	}
	a, _ := f.store.Get(userA)
	if a == nil || a.AcceptedAt.Year() < 2026 {
		t.Errorf("accepted_at must be the server's, not the client's: %+v", a)
	}
}

func TestEachUserAcceptsOnlyForThemselves(t *testing.T) {
	f := newTermsFixture(t)
	code(t, f.accept("tok-a", "1.0"), http.StatusOK)
	code(t, f.accept("tok-b", "2.0"), http.StatusOK)
	a, _ := f.store.Get(userA)
	b, _ := f.store.Get(userB)
	if a.Version != "1.0" || b.Version != "2.0" {
		t.Errorf("a = %+v, b = %+v", a, b)
	}
}

func TestInvalidVersionsAreRejected(t *testing.T) {
	f := newTermsFixture(t)
	for _, body := range []string{`{}`, `{"version":""}`, `{"version":"v1"}`, `{"version":"1.0-beta"}`, `{"version":"01.0"}`, `{"version":1.0}`, `not json`} {
		rec := f.do(http.MethodPut, "/profile/terms", "tok-a", body)
		code(t, rec, http.StatusBadRequest)
		if body != `not json` && !strings.Contains(rec.Body.String(), "invalid_version") && body != `{"version":1.0}` {
			t.Errorf("body %q -> %s, want error invalid_version", body, rec.Body)
		}
	}
	if a, _ := f.store.Get(userA); a != nil {
		t.Errorf("a rejected request wrote %+v", a)
	}
}

func TestSameVersionAgainIsIdempotentAndKeepsTheOriginalTime(t *testing.T) {
	f := newTermsFixture(t)
	first := decodeAccepted(t, f.accept("tok-a", "1.0"))
	time.Sleep(5 * time.Millisecond)
	rec := f.accept("tok-a", "1.0")
	code(t, rec, http.StatusOK)
	if again := decodeAccepted(t, rec); !again.AcceptedAt.Equal(first.AcceptedAt) {
		t.Errorf("accepted_at moved from %v to %v", first.AcceptedAt, again.AcceptedAt)
	}
}

func TestOlderVersionIsA409WithTheCurrentVersion(t *testing.T) {
	f := newTermsFixture(t)
	code(t, f.accept("tok-a", "1.2"), http.StatusOK)
	for _, older := range []string{"1.1", "1.0", "0.9.9"} {
		rec := f.accept("tok-a", older)
		code(t, rec, http.StatusConflict)
		var body struct {
			Error          string `json:"error"`
			CurrentVersion string `json:"current_version"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body.Error != "terms_version_not_newer" || body.CurrentVersion != "1.2" {
			t.Errorf("%s: body = %s", older, rec.Body)
		}
	}
	if a, _ := f.store.Get(userA); a.Version != "1.2" {
		t.Errorf("stored = %+v", a)
	}
}

func TestNewerVersionReplacesTheRecord(t *testing.T) {
	f := newTermsFixture(t)
	first := decodeAccepted(t, f.accept("tok-a", "1.9"))
	time.Sleep(5 * time.Millisecond)
	rec := f.accept("tok-a", "1.10")
	code(t, rec, http.StatusOK)
	got := decodeAccepted(t, rec)
	if got.Version != "1.10" || !got.AcceptedAt.After(first.AcceptedAt) {
		t.Errorf("1.10 is newer than 1.9: %+v after %+v", got, first)
	}
}

func TestAcceptingWithoutAProfileIs404(t *testing.T) {
	f := newTermsFixture(t)
	code(t, f.accept("tok-nobody", "1.0"), http.StatusNotFound)
}

func TestProfileReadCarriesTermsAccepted(t *testing.T) {
	f := newTermsFixture(t)
	rec := f.do(http.MethodGet, "/profile", "tok-a", "")
	code(t, rec, http.StatusOK)
	var before map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &before)
	if v, present := before["terms_accepted"]; !present || v != nil {
		t.Errorf("before accepting, terms_accepted should be present and null: %s", rec.Body)
	}

	accepted := decodeAccepted(t, f.accept("tok-a", "1.0"))
	rec = f.do(http.MethodGet, "/profile", "tok-a", "")
	var after struct {
		TermsAccepted *models.TermsAccepted `json:"terms_accepted"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.TermsAccepted == nil || after.TermsAccepted.Version != "1.0" || !after.TermsAccepted.AcceptedAt.Equal(accepted.AcceptedAt) {
		t.Errorf("profile = %s", rec.Body)
	}

	rec = f.do(http.MethodGet, "/profile", "tok-b", "")
	if strings.Contains(rec.Body.String(), `"version":"1.0"`) {
		t.Errorf("user B sees user A's acceptance: %s", rec.Body)
	}
}
