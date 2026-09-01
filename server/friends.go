package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

type sendFriendRequestRequest struct {
	// Identifier is a User.id, an email, or a username - resolved server-side to the target
	// user (see models.ResolveFriendTarget).
	Identifier string `json:"identifier"`
}

type friendRequestResponse struct {
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	Status string    `json:"status"`
}

type friendEntry struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Since  time.Time `json:"since"`
}

type friendsResponse struct {
	Friends []friendEntry `json:"friends"`
}

type pendingRequestEntry struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type pendingRequestsResponse struct {
	Incoming []pendingRequestEntry `json:"incoming"`
	Outgoing []pendingRequestEntry `json:"outgoing"`
}

func setupFriendsHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up self-service friends endpoint handlers...")

	g.GET("/friends", listFriendsHandler(authzClient))
	g.GET("/friends/requests", listFriendRequestsHandler(authzClient))
	g.POST("/friends/requests", sendFriendRequestHandler(authzClient))
	g.POST("/friends/requests/:id/accept", respondFriendRequestHandler(authzClient, models.AcceptFriendRequest))
	g.POST("/friends/requests/:id/decline", respondFriendRequestHandler(authzClient, models.DeclineFriendRequest))
	g.DELETE("/friends/:id", removeFriendHandler(authzClient))
}

// resolveCallerUserID resolves the verified caller to their own User.id, writing the standard
// 401/404/503 responses and returning ok=false on failure so handlers can `if !ok { return }`.
func resolveCallerUserID(c *gin.Context, authzClient *authz.Client) (uuid.UUID, bool) {
	subject, ok := resolveVerifiedSubject(c, authzClient)
	if !ok {
		return uuid.UUID{}, false
	}
	return callerUserIDFromSubject(c, subject)
}

// callerUserIDFromSubject resolves an already-verified Auth0 subject to its own User.id,
// writing the standard 404/500 responses and returning ok=false on failure.
func callerUserIDFromSubject(c *gin.Context, subject string) (uuid.UUID, bool) {
	profile, err := models.FindProfileBySubject(c.Request.Context(), subject)
	if err != nil {
		if err == models.ErrProfileNotFound {
			c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
			return uuid.UUID{}, false
		}
		logging.Logger.Error("Failed to resolve caller profile", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to resolve caller"})
		return uuid.UUID{}, false
	}
	return profile.UserID, true
}

// friendshipIDParam parses the :id path parameter as a UUID, writing a 404 and returning
// ok=false if it isn't one (an unparseable id can't match any friendship).
func friendshipIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "friendship not found"})
		return uuid.UUID{}, false
	}
	return id, true
}

// writeFriendshipError maps a models friendship-flow error to its HTTP status.
func writeFriendshipError(c *gin.Context, action string, err error) {
	switch err {
	case models.ErrFriendshipNotFound:
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "friendship not found"})
	case models.ErrNotAParty, models.ErrCannotRespondOwnRequest:
		c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "not allowed to " + action + " this request"})
	case models.ErrRequestNotPending:
		c.JSON(http.StatusConflict, apiv.ErrorVO{Error: "conflict", Message: "friendship is not in a state that can be " + action + "d"})
	default:
		logging.Logger.Error("friendship "+action+" failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to " + action + " request"})
	}
}

// Send a friend request to another user.
//
//	 The target is given as an identifier - a User.id, an email, or a username - resolved
//	 server-side. The caller is resolved from their own verified Auth0 subject, never a
//	 client-supplied value.
//		@Summary		Send a friend request
//		@Description	Send a friend request to another user, identified by User.id, email, or username
//		@Tags			friends
//		@Accept			json
//		@Produce		json
//		@Param			request	body		sendFriendRequestRequest	true	"Target user identifier"
//		@Success		201		{object}	friendRequestResponse
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		404		{object}	apiv.ErrorVO
//		@Failure		409		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/friends/requests [post]
func sendFriendRequestHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		var req sendFriendRequestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}
		if req.Identifier == "" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "identifier is required"})
			return
		}

		caller, ok := callerUserIDFromSubject(c, subject)
		if !ok {
			return
		}

		target, err := models.ResolveFriendTarget(c.Request.Context(), req.Identifier)
		if err != nil {
			if err == models.ErrTargetNotFound || err == models.ErrUserNotFound {
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no user found for that id, email, or username"})
				return
			}
			logging.Logger.Error("Failed to resolve friend target", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to send friend request"})
			return
		}

		friendship, err := models.SendFriendRequest(c.Request.Context(), caller, target)
		if err != nil {
			switch err {
			case models.ErrSelfRequest:
				c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "cannot send a friend request to yourself"})
			case models.ErrDuplicateRequest:
				c.JSON(http.StatusConflict, apiv.ErrorVO{Error: "conflict", Message: "a friend request or friendship already exists for this user"})
			default:
				logging.Logger.Error("Failed to send friend request", "error", err.Error())
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to send friend request"})
			}
			return
		}

		c.JSON(http.StatusCreated, friendRequestResponse{ID: friendship.ID, UserID: target, Status: friendship.Status})
	}
}

// Respond to a pending friend request (accept or decline).
//
//	 The caller must be a party to the request and must not be the user who sent it.
//		@Summary		Respond to a pending friend request
//		@Description	Accept or decline a pending friend request the caller received
//		@Tags			friends
//		@Produce		json
//		@Param			id	path		string	true	"Friendship id"
//		@Success		204
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		403	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		409	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/friends/requests/{id}/accept [post]
func respondFriendRequestHandler(authzClient *authz.Client, respond func(context.Context, uuid.UUID, uuid.UUID) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := resolveCallerUserID(c, authzClient)
		if !ok {
			return
		}
		id, ok := friendshipIDParam(c)
		if !ok {
			return
		}

		if err := respond(c.Request.Context(), caller, id); err != nil {
			writeFriendshipError(c, "respond to", err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// Remove an existing friendship.
//
//	 Either party to an accepted friendship may remove it.
//		@Summary		Remove a friendship
//		@Description	Remove an existing accepted friendship
//		@Tags			friends
//		@Produce		json
//		@Param			id	path		string	true	"Friendship id"
//		@Success		204
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		403	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		409	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/friends/{id} [delete]
func removeFriendHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := resolveCallerUserID(c, authzClient)
		if !ok {
			return
		}
		id, ok := friendshipIDParam(c)
		if !ok {
			return
		}

		if err := models.RemoveFriendship(c.Request.Context(), caller, id); err != nil {
			writeFriendshipError(c, "remove", err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// List the caller's current friends.
//
//	@Summary		List friends
//	@Description	List the caller's accepted mutual friendships
//	@Tags			friends
//	@Produce		json
//	@Success		200	{object}	friendsResponse
//	@Failure		401	{object}	apiv.ErrorVO
//	@Failure		404	{object}	apiv.ErrorVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/friends [get]
func listFriendsHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := resolveCallerUserID(c, authzClient)
		if !ok {
			return
		}

		friends, err := models.ListFriends(c.Request.Context(), caller)
		if err != nil {
			logging.Logger.Error("Failed to list friends", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list friends"})
			return
		}

		entries := make([]friendEntry, 0, len(friends))
		for _, f := range friends {
			entries = append(entries, friendEntry{UserID: f.UserID, Name: f.Name, Email: f.Email, Since: f.Since})
		}
		c.JSON(http.StatusOK, friendsResponse{Friends: entries})
	}
}

// List the caller's pending friend requests, sent and received.
//
//	@Summary		List pending friend requests
//	@Description	List the caller's pending friend requests, split into incoming and outgoing
//	@Tags			friends
//	@Produce		json
//	@Success		200	{object}	pendingRequestsResponse
//	@Failure		401	{object}	apiv.ErrorVO
//	@Failure		404	{object}	apiv.ErrorVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/friends/requests [get]
func listFriendRequestsHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := resolveCallerUserID(c, authzClient)
		if !ok {
			return
		}

		requests, err := models.ListPendingRequests(c.Request.Context(), caller)
		if err != nil {
			logging.Logger.Error("Failed to list friend requests", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list friend requests"})
			return
		}

		resp := pendingRequestsResponse{
			Incoming: make([]pendingRequestEntry, 0),
			Outgoing: make([]pendingRequestEntry, 0),
		}
		for _, r := range requests {
			entry := pendingRequestEntry{ID: r.ID, UserID: r.UserID, Name: r.Name, Email: r.Email, CreatedAt: r.CreatedAt}
			if r.Outgoing {
				resp.Outgoing = append(resp.Outgoing, entry)
			} else {
				resp.Incoming = append(resp.Incoming, entry)
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}
