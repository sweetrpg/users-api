package models

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Friendship-flow errors. Each maps to a distinct HTTP status in the handler layer; the model
// never inspects the caller's identity beyond the party/requestedBy checks encoded here.
var (
	// ErrSelfRequest means the caller targeted their own User.id.
	ErrSelfRequest = errors.New("models: cannot send a friend request to yourself")
	// ErrDuplicateRequest means a pending or accepted friendship already exists for the pair,
	// in either direction (the canonical-pair unique index rejected the insert).
	ErrDuplicateRequest = errors.New("models: a friend request or friendship already exists for this pair")
	// ErrFriendshipNotFound means no friendship document has the given id.
	ErrFriendshipNotFound = errors.New("models: friendship not found")
	// ErrNotAParty means the friendship exists but the caller is neither userA nor userB.
	ErrNotAParty = errors.New("models: caller is not a party to this friendship")
	// ErrCannotRespondOwnRequest means the caller is the original requester and so cannot
	// accept or decline their own outgoing request.
	ErrCannotRespondOwnRequest = errors.New("models: cannot accept or decline your own request")
	// ErrRequestNotPending means accept/decline was attempted on an already-accepted
	// friendship, or remove on a still-pending one.
	ErrRequestNotPending = errors.New("models: friendship is not in the required state")
)

// friendshipDoc is the friendships collection document shape. userA/userB are stored in a
// canonical order (userA is the lexicographically smaller id) so the unique index on
// (userA, userB) rejects a duplicate request sent in either direction - see
// add-users-api-friends design.md.
type friendshipDoc struct {
	ID          uuid.UUID `bson:"_id"`
	UserA       uuid.UUID `bson:"userA"`
	UserB       uuid.UUID `bson:"userB"`
	Status      string    `bson:"status"`
	RequestedBy uuid.UUID `bson:"requestedBy"`
	CreatedAt   time.Time `bson:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

// Friend is one entry in a caller's accepted-friend list: the other party's id, plus their
// profile name/email when available.
type Friend struct {
	UserID uuid.UUID
	Name   string
	Email  string
	Since  time.Time
}

// FriendRequest is one pending request involving the caller. Outgoing is true when the caller
// is the one who sent it.
type FriendRequest struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Email     string
	Outgoing  bool
	CreatedAt time.Time
}

// orderPair returns x and y sorted so the first return value is the lexicographically smaller
// of the two 16-byte ids - the canonical (userA, userB) ordering every friendship write uses.
func orderPair(x, y uuid.UUID) (a, b uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}

// EnsureFriendshipIndexes creates the unique index on friendships.(userA, userB) that
// SendFriendRequest relies on to reject a duplicate request via a duplicate-key error rather
// than a read-then-write check. Safe to call on every startup - CreateOne is idempotent for an
// identical index definition.
func EnsureFriendshipIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.FriendshipsCollection)
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "userA", Value: 1},
			{Key: "userB", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// SendFriendRequest creates a pending friendship from caller to target. Rejects a self-request
// and, via the unique index, a duplicate for a pair that already has a pending request or an
// accepted friendship in either direction.
func SendFriendRequest(ctx context.Context, caller, target uuid.UUID) (*friendshipDoc, error) {
	if caller == target {
		return nil, ErrSelfRequest
	}

	userA, userB := orderPair(caller, target)
	now := time.Now().UTC()
	doc := friendshipDoc{
		ID:          uuid.New(),
		UserA:       userA,
		UserB:       userB,
		Status:      constants.FriendshipStatusPending,
		RequestedBy: caller,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := database.Db.Collection(constants.FriendshipsCollection).InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDuplicateRequest
		}
		return nil, err
	}
	return &doc, nil
}

// AcceptFriendRequest marks a pending request accepted. The caller must be a party to the
// document and must not be the original requester. The state check is a single atomic
// UpdateOne; a follow-up owner-agnostic read only runs to classify a miss (404 vs 403 vs 409)
// and never re-opens a window to bypass the check.
func AcceptFriendRequest(ctx context.Context, caller, id uuid.UUID) error {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "status", Value: constants.FriendshipStatusPending},
		{Key: "requestedBy", Value: bson.D{{Key: "$ne", Value: caller}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "userA", Value: caller}},
			bson.D{{Key: "userB", Value: caller}},
		}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "status", Value: constants.FriendshipStatusAccepted},
		{Key: "updatedAt", Value: time.Now().UTC()},
	}}}

	result, err := database.Db.Collection(constants.FriendshipsCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return classifyRespondMiss(ctx, caller, id, constants.FriendshipStatusPending)
	}
	return nil
}

// DeclineFriendRequest deletes a pending request. Same caller checks as
// AcceptFriendRequest - only the recipient may decline. Declining fully clears the pair so a
// future request can be sent (the document is removed, not soft-deleted).
func DeclineFriendRequest(ctx context.Context, caller, id uuid.UUID) error {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "status", Value: constants.FriendshipStatusPending},
		{Key: "requestedBy", Value: bson.D{{Key: "$ne", Value: caller}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "userA", Value: caller}},
			bson.D{{Key: "userB", Value: caller}},
		}},
	}

	result, err := database.Db.Collection(constants.FriendshipsCollection).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return classifyRespondMiss(ctx, caller, id, constants.FriendshipStatusPending)
	}
	return nil
}

// RemoveFriendship deletes an accepted friendship. Either party may remove it. A new request
// can be sent afterward since the document is gone.
func RemoveFriendship(ctx context.Context, caller, id uuid.UUID) error {
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "status", Value: constants.FriendshipStatusAccepted},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "userA", Value: caller}},
			bson.D{{Key: "userB", Value: caller}},
		}},
	}

	result, err := database.Db.Collection(constants.FriendshipsCollection).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return classifyRespondMiss(ctx, caller, id, constants.FriendshipStatusAccepted)
	}
	return nil
}

// classifyRespondMiss runs one owner-agnostic lookup by id to explain why an atomic
// owner-scoped write matched nothing. wantStatus is the status the write required.
func classifyRespondMiss(ctx context.Context, caller, id uuid.UUID, wantStatus string) error {
	var doc friendshipDoc
	err := database.Db.Collection(constants.FriendshipsCollection).
		FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrFriendshipNotFound
		}
		return err
	}
	if doc.UserA != caller && doc.UserB != caller {
		return ErrNotAParty
	}
	if doc.Status != wantStatus {
		return ErrRequestNotPending
	}
	// Party, right status, but the write still missed - only remaining cause is the
	// requestedBy != caller guard on accept/decline.
	if doc.RequestedBy == caller {
		return ErrCannotRespondOwnRequest
	}
	return ErrFriendshipNotFound
}

// ListFriends returns the caller's accepted friendships as the other party's id, enriched with
// profile name/email when a users document exists.
func ListFriends(ctx context.Context, caller uuid.UUID) ([]Friend, error) {
	docs, err := database.Query[friendshipDoc](constants.FriendshipsCollection, bson.D{
		{Key: "status", Value: constants.FriendshipStatusAccepted},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "userA", Value: caller}},
			bson.D{{Key: "userB", Value: caller}},
		}},
	}, nil, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	friends := make([]Friend, 0, len(docs))
	otherIDs := make([]uuid.UUID, 0, len(docs))
	for _, d := range docs {
		other := d.UserB
		if d.UserA != caller {
			other = d.UserA
		}
		otherIDs = append(otherIDs, other)
		friends = append(friends, Friend{UserID: other, Since: d.UpdatedAt})
	}

	names := lookupUserSummaries(ctx, otherIDs)
	for i := range friends {
		if s, ok := names[friends[i].UserID]; ok {
			friends[i].Name = s.name
			friends[i].Email = s.email
		}
	}
	return friends, nil
}

// ListPendingRequests returns every pending request the caller has sent or received, with
// Outgoing set on the ones they sent.
func ListPendingRequests(ctx context.Context, caller uuid.UUID) ([]FriendRequest, error) {
	docs, err := database.Query[friendshipDoc](constants.FriendshipsCollection, bson.D{
		{Key: "status", Value: constants.FriendshipStatusPending},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "userA", Value: caller}},
			bson.D{{Key: "userB", Value: caller}},
		}},
	}, nil, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	requests := make([]FriendRequest, 0, len(docs))
	otherIDs := make([]uuid.UUID, 0, len(docs))
	for _, d := range docs {
		other := d.UserB
		if d.UserA != caller {
			other = d.UserA
		}
		otherIDs = append(otherIDs, other)
		requests = append(requests, FriendRequest{
			ID:        d.ID,
			UserID:    other,
			Outgoing:  d.RequestedBy == caller,
			CreatedAt: d.CreatedAt,
		})
	}

	names := lookupUserSummaries(ctx, otherIDs)
	for i := range requests {
		if s, ok := names[requests[i].UserID]; ok {
			requests[i].Name = s.name
			requests[i].Email = s.email
		}
	}
	return requests, nil
}

type userSummary struct {
	name  string
	email string
}

// lookupUserSummaries best-effort resolves ids to name/email from the users collection. A
// missing document (or a decode failure on a legacy id shape) just yields no entry - callers
// treat the enrichment as optional.
func lookupUserSummaries(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]userSummary {
	out := make(map[uuid.UUID]userSummary, len(ids))
	if len(ids) == 0 {
		return out
	}

	values := bson.A{}
	for _, id := range ids {
		values = append(values, id)
	}
	filter := bson.D{{Key: "$and", Value: bson.A{
		notSoftDeletedFilter,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: values}}}},
	}}}

	docs, err := database.Query[userDoc](constants.UsersCollection, filter, nil, nil, 0, 0)
	if err != nil {
		return out
	}
	for _, d := range docs {
		out[d.ID] = userSummary{name: d.Name, email: d.Email}
	}
	return out
}
