package models

import (
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

// linkTicketTTL is the account-linking ticket lifetime (see design.md's "5-minute TTL" decision).
// Enforced twice: the Mongo TTL index below reaps expired documents in the background, and
// consumeLinkTicket re-checks CreatedAt explicitly so a ticket is rejected as expired even in the
// window before the TTL background job has run.
const linkTicketTTL = 5 * time.Minute

// ErrTicketNotFound means the ticket id doesn't match any link_tickets document - never minted,
// or already reaped by the TTL index.
var ErrTicketNotFound = errors.New("models: link ticket not found")

// ErrTicketExpired means the ticket exists but is older than linkTicketTTL.
var ErrTicketExpired = errors.New("models: link ticket expired")

// ErrTicketConsumed means the ticket has already been used by an earlier link/complete call.
var ErrTicketConsumed = errors.New("models: link ticket already consumed")

// ErrLinkConflict means the identity being linked already has a LoginProfile attached to a
// different User than the one the ticket targets - see design.md's "reject, never merge" rule.
var ErrLinkConflict = errors.New("models: identity already linked to a different user")

// linkTicketDoc is the link_tickets collection document shape: a short-lived, single-use token
// binding a link-completion request to the User.id it's allowed to attach an identity to (see
// design.md's "server-signed ... link ticket" decision).
type linkTicketDoc struct {
	ID         uuid.UUID  `bson:"_id"`
	UserID     uuid.UUID  `bson:"userId"`
	CreatedAt  time.Time  `bson:"createdAt"`
	ConsumedAt *time.Time `bson:"consumedAt,omitempty"`
}

// EnsureLinkTicketIndexes creates the TTL index on link_tickets.createdAt that expires documents
// linkTicketTTL after creation. Safe to call on every startup - CreateOne is idempotent for an
// identical index definition.
func EnsureLinkTicketIndexes(ctx context.Context) error {
	collection := database.Db.Collection(constants.LinkTicketsCollection)
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "createdAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(linkTicketTTL.Seconds())),
	})
	return err
}

// MintLinkTicket creates a new, unconsumed link ticket bound to userID and returns its id.
func MintLinkTicket(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	ticket := linkTicketDoc{ID: uuid.New(), UserID: userID, CreatedAt: time.Now().UTC()}
	if _, err := database.Db.Collection(constants.LinkTicketsCollection).InsertOne(ctx, ticket); err != nil {
		return uuid.UUID{}, err
	}
	return ticket.ID, nil
}

// consumeLinkTicket atomically marks ticketID consumed (matching only a not-yet-consumed
// document, so a concurrent second call never succeeds) and returns the ticket as it was just
// before consumption. Distinguishes ErrTicketNotFound/ErrTicketConsumed with a follow-up
// unconditional lookup only on the failure path, so the common success path costs one round trip.
func consumeLinkTicket(ctx context.Context, ticketID uuid.UUID, now time.Time) (*linkTicketDoc, error) {
	collection := database.Db.Collection(constants.LinkTicketsCollection)
	filter := bson.D{
		{Key: "_id", Value: ticketID},
		{Key: "consumedAt", Value: bson.D{{Key: "$exists", Value: false}}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "consumedAt", Value: now}}}}

	var ticket linkTicketDoc
	err := collection.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.Before),
	).Decode(&ticket)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		var existing linkTicketDoc
		lookupErr := collection.FindOne(ctx, bson.D{{Key: "_id", Value: ticketID}}).Decode(&existing)
		if errors.Is(lookupErr, mongo.ErrNoDocuments) {
			return nil, ErrTicketNotFound
		}
		if lookupErr != nil {
			return nil, lookupErr
		}
		return nil, ErrTicketConsumed
	}

	if now.Sub(ticket.CreatedAt) > linkTicketTTL {
		return nil, ErrTicketExpired
	}
	return &ticket, nil
}
