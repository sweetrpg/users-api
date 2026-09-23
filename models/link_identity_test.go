package models

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sweetrpg/mongodb.go/database"
	"github.com/sweetrpg/users-api/constants"
	"go.mongodb.org/mongo-driver/bson"
)

func seedTicket(t *testing.T, userID uuid.UUID, createdAt time.Time) uuid.UUID {
	t.Helper()
	ticket := linkTicketDoc{ID: uuid.New(), UserID: userID, CreatedAt: createdAt}
	if _, err := database.Db.Collection(constants.LinkTicketsCollection).InsertOne(context.Background(), ticket); err != nil {
		t.Fatalf("seeding link ticket: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(constants.LinkTicketsCollection).DeleteOne(context.Background(),
			bson.D{{Key: "_id", Value: ticket.ID}})
	})
	return ticket.ID
}

func TestLinkIdentity_NewSubjectCreatesLoginProfile(t *testing.T) {
	userID := uuid.New()
	subject := "auth0|test-link-new-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })
	ticketID := seedTicket(t, userID, time.Now().UTC())

	result, err := LinkIdentity(context.Background(), subject, ticketID)
	if err != nil {
		t.Fatalf("LinkIdentity: %v", err)
	}
	if !result.Created {
		t.Errorf("Created = false, want true for a new subject")
	}
	if result.UserID != userID {
		t.Errorf("UserID = %v, want %v", result.UserID, userID)
	}
}

func TestLinkIdentity_IdempotentReLink(t *testing.T) {
	userID := uuid.New()
	subject := "auth0|test-link-idempotent-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	firstTicket := seedTicket(t, userID, time.Now().UTC())
	first, err := LinkIdentity(context.Background(), subject, firstTicket)
	if err != nil {
		t.Fatalf("first LinkIdentity: %v", err)
	}
	if !first.Created {
		t.Fatalf("first Created = false, want true")
	}

	secondTicket := seedTicket(t, userID, time.Now().UTC())
	second, err := LinkIdentity(context.Background(), subject, secondTicket)
	if err != nil {
		t.Fatalf("second LinkIdentity: %v", err)
	}
	if second.Created {
		t.Errorf("second Created = true, want false (idempotent re-link)")
	}
	if second.UserID != userID {
		t.Errorf("second UserID = %v, want %v", second.UserID, userID)
	}
}

func TestLinkIdentity_ConflictOnDifferentUser(t *testing.T) {
	originalUserID := uuid.New()
	otherUserID := uuid.New()
	subject := "auth0|test-link-conflict-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	firstTicket := seedTicket(t, originalUserID, time.Now().UTC())
	if _, err := LinkIdentity(context.Background(), subject, firstTicket); err != nil {
		t.Fatalf("first LinkIdentity: %v", err)
	}

	conflictTicket := seedTicket(t, otherUserID, time.Now().UTC())
	_, err := LinkIdentity(context.Background(), subject, conflictTicket)
	if !errors.Is(err, ErrLinkConflict) {
		t.Errorf("err = %v, want ErrLinkConflict", err)
	}
}

func TestLinkIdentity_ExpiredTicketRejected(t *testing.T) {
	userID := uuid.New()
	subject := "auth0|test-link-expired-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	ticketID := seedTicket(t, userID, time.Now().UTC().Add(-linkTicketTTL-time.Minute))

	_, err := LinkIdentity(context.Background(), subject, ticketID)
	if !errors.Is(err, ErrTicketExpired) {
		t.Errorf("err = %v, want ErrTicketExpired", err)
	}
}

func TestLinkIdentity_AlreadyConsumedTicketRejected(t *testing.T) {
	userID := uuid.New()
	subject := "auth0|test-link-consumed-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	ticketID := seedTicket(t, userID, time.Now().UTC())
	if _, err := LinkIdentity(context.Background(), subject, ticketID); err != nil {
		t.Fatalf("first LinkIdentity: %v", err)
	}

	_, err := LinkIdentity(context.Background(), subject, ticketID)
	if !errors.Is(err, ErrTicketConsumed) {
		t.Errorf("err = %v, want ErrTicketConsumed", err)
	}
}

func TestLinkIdentity_UnknownTicketRejected(t *testing.T) {
	subject := "auth0|test-link-unknown-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	_, err := LinkIdentity(context.Background(), subject, uuid.New())
	if !errors.Is(err, ErrTicketNotFound) {
		t.Errorf("err = %v, want ErrTicketNotFound", err)
	}
}

func TestUnlinkIdentity_BelowOneRejected(t *testing.T) {
	userID := uuid.New()
	subject := "auth0|test-unlink-below-one-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subject) })

	ticketID := seedTicket(t, userID, time.Now().UTC())
	if _, err := LinkIdentity(context.Background(), subject, ticketID); err != nil {
		t.Fatalf("LinkIdentity: %v", err)
	}

	identities, err := ListLinkedIdentities(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListLinkedIdentities: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("len(identities) = %d, want 1", len(identities))
	}

	err = UnlinkIdentity(context.Background(), userID, identities[0].LoginProfileID)
	if !errors.Is(err, ErrLastLoginMethod) {
		t.Errorf("err = %v, want ErrLastLoginMethod", err)
	}
}

func TestUnlinkIdentity_AboveOneSucceeds(t *testing.T) {
	userID := uuid.New()
	subjectA := "auth0|test-unlink-above-one-a-" + t.Name()
	subjectB := "github|test-unlink-above-one-b-" + t.Name()
	t.Cleanup(func() { cleanupSubject(t, subjectA) })
	t.Cleanup(func() { cleanupSubject(t, subjectB) })

	ticketA := seedTicket(t, userID, time.Now().UTC())
	if _, err := LinkIdentity(context.Background(), subjectA, ticketA); err != nil {
		t.Fatalf("LinkIdentity A: %v", err)
	}
	ticketB := seedTicket(t, userID, time.Now().UTC())
	if _, err := LinkIdentity(context.Background(), subjectB, ticketB); err != nil {
		t.Fatalf("LinkIdentity B: %v", err)
	}

	identities, err := ListLinkedIdentities(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListLinkedIdentities: %v", err)
	}
	if len(identities) != 2 {
		t.Fatalf("len(identities) = %d, want 2", len(identities))
	}

	if err := UnlinkIdentity(context.Background(), userID, identities[0].LoginProfileID); err != nil {
		t.Fatalf("UnlinkIdentity: %v", err)
	}

	remaining, err := ListLinkedIdentities(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListLinkedIdentities after unlink: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("len(remaining) = %d, want 1", len(remaining))
	}
}
