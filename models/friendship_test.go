package models

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

// cleanupFriendships removes any friendship document touching one of the given ids, so a
// failed run doesn't leave state that trips the unique index on the next run.
func cleanupFriendships(t *testing.T, ids ...uuid.UUID) {
	t.Helper()
	values := bson.A{}
	for _, id := range ids {
		values = append(values, id)
	}
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "userA", Value: bson.D{{Key: "$in", Value: values}}}},
		bson.D{{Key: "userB", Value: bson.D{{Key: "$in", Value: values}}}},
	}}}
	_, _ = database.Db.Collection(constants.FriendshipsCollection).DeleteMany(context.Background(), filter)
}

func newPair(t *testing.T) (a, b uuid.UUID) {
	t.Helper()
	a, b = uuid.New(), uuid.New()
	cleanupFriendships(t, a, b)
	t.Cleanup(func() { cleanupFriendships(t, a, b) })
	return a, b
}

func TestSendFriendRequest_CreatesPendingNotFriendship(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	if _, err := SendFriendRequest(ctx, a, b); err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}

	friends, err := ListFriends(ctx, b)
	if err != nil {
		t.Fatalf("ListFriends: %v", err)
	}
	if len(friends) != 0 {
		t.Errorf("ListFriends(b) = %d entries, want 0 (pending is not a friendship)", len(friends))
	}

	incoming, err := ListPendingRequests(ctx, b)
	if err != nil {
		t.Fatalf("ListPendingRequests: %v", err)
	}
	if len(incoming) != 1 || incoming[0].Outgoing || incoming[0].UserID != a {
		t.Errorf("recipient pending = %+v, want one incoming request from %s", incoming, a)
	}

	outgoing, err := ListPendingRequests(ctx, a)
	if err != nil {
		t.Fatalf("ListPendingRequests: %v", err)
	}
	if len(outgoing) != 1 || !outgoing[0].Outgoing || outgoing[0].UserID != b {
		t.Errorf("sender pending = %+v, want one outgoing request to %s", outgoing, b)
	}
}

func TestSendFriendRequest_RejectsSelf(t *testing.T) {
	a := uuid.New()
	if _, err := SendFriendRequest(context.Background(), a, a); !errors.Is(err, ErrSelfRequest) {
		t.Errorf("err = %v, want ErrSelfRequest", err)
	}
}

func TestSendFriendRequest_RejectsDuplicateEitherDirection(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	if _, err := SendFriendRequest(ctx, a, b); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if _, err := SendFriendRequest(ctx, a, b); !errors.Is(err, ErrDuplicateRequest) {
		t.Errorf("same-direction duplicate err = %v, want ErrDuplicateRequest", err)
	}
	if _, err := SendFriendRequest(ctx, b, a); !errors.Is(err, ErrDuplicateRequest) {
		t.Errorf("reverse-direction duplicate err = %v, want ErrDuplicateRequest", err)
	}
}

func TestAccept_EstablishesMutualFriendship(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if err := AcceptFriendRequest(ctx, b, fr.ID); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}

	for _, party := range []uuid.UUID{a, b} {
		friends, err := ListFriends(ctx, party)
		if err != nil {
			t.Fatalf("ListFriends(%s): %v", party, err)
		}
		if len(friends) != 1 {
			t.Fatalf("ListFriends(%s) = %d, want 1", party, len(friends))
		}
		other := b
		if party == b {
			other = a
		}
		if friends[0].UserID != other {
			t.Errorf("ListFriends(%s)[0].UserID = %s, want %s", party, friends[0].UserID, other)
		}
		pending, _ := ListPendingRequests(ctx, party)
		if len(pending) != 0 {
			t.Errorf("ListPendingRequests(%s) = %d, want 0 after accept", party, len(pending))
		}
	}
}

func TestAccept_RejectsThirdPartyAndOriginalRequester(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)
	c := uuid.New()

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}

	if err := AcceptFriendRequest(ctx, c, fr.ID); !errors.Is(err, ErrNotAParty) {
		t.Errorf("third-party accept err = %v, want ErrNotAParty", err)
	}
	if err := AcceptFriendRequest(ctx, a, fr.ID); !errors.Is(err, ErrCannotRespondOwnRequest) {
		t.Errorf("requester accept err = %v, want ErrCannotRespondOwnRequest", err)
	}
	if err := DeclineFriendRequest(ctx, a, fr.ID); !errors.Is(err, ErrCannotRespondOwnRequest) {
		t.Errorf("requester decline err = %v, want ErrCannotRespondOwnRequest", err)
	}
}

func TestAccept_UnknownIDIsNotFound(t *testing.T) {
	if err := AcceptFriendRequest(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrFriendshipNotFound) {
		t.Errorf("err = %v, want ErrFriendshipNotFound", err)
	}
}

func TestDecline_ClearsPendingAndAllowsReRequest(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if err := DeclineFriendRequest(ctx, b, fr.ID); err != nil {
		t.Fatalf("DeclineFriendRequest: %v", err)
	}

	if pending, _ := ListPendingRequests(ctx, b); len(pending) != 0 {
		t.Errorf("pending after decline = %d, want 0", len(pending))
	}
	if _, err := SendFriendRequest(ctx, a, b); err != nil {
		t.Errorf("re-request after decline: %v, want nil", err)
	}
}

func TestRemove_EndsFriendshipAndAllowsReRequest(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if err := AcceptFriendRequest(ctx, b, fr.ID); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	if err := RemoveFriendship(ctx, a, fr.ID); err != nil {
		t.Fatalf("RemoveFriendship: %v", err)
	}

	for _, party := range []uuid.UUID{a, b} {
		if friends, _ := ListFriends(ctx, party); len(friends) != 0 {
			t.Errorf("ListFriends(%s) = %d after remove, want 0", party, len(friends))
		}
	}
	if _, err := SendFriendRequest(ctx, b, a); err != nil {
		t.Errorf("re-request after remove: %v, want nil", err)
	}
}

func TestRemove_RejectsNonPartyAndPending(t *testing.T) {
	ctx := context.Background()
	a, b := newPair(t)
	c := uuid.New()

	fr, err := SendFriendRequest(ctx, a, b)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}

	// Still pending - remove is only for accepted friendships.
	if err := RemoveFriendship(ctx, a, fr.ID); !errors.Is(err, ErrRequestNotPending) {
		t.Errorf("remove pending err = %v, want ErrRequestNotPending", err)
	}

	if err := AcceptFriendRequest(ctx, b, fr.ID); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	if err := RemoveFriendship(ctx, c, fr.ID); !errors.Is(err, ErrNotAParty) {
		t.Errorf("third-party remove err = %v, want ErrNotAParty", err)
	}
}

func TestOrderPair_IsCanonical(t *testing.T) {
	x, y := uuid.New(), uuid.New()
	a1, b1 := orderPair(x, y)
	a2, b2 := orderPair(y, x)
	if a1 != a2 || b1 != b2 {
		t.Errorf("orderPair not canonical: (%s,%s) vs (%s,%s)", a1, b1, a2, b2)
	}
}
