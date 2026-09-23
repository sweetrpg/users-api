package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

type linkStartResponse struct {
	Ticket string `json:"ticket"`
}

type linkCompleteRequest struct {
	Ticket string `json:"ticket"`
}

type linkCompleteResponse struct {
	UserID  string `json:"userId"`
	Created bool   `json:"created"`
}

type linkedIdentityResponse struct {
	LoginProfileID string `json:"loginProfileId"`
	ConnectionType string `json:"connectionType"`
	LinkedAt       string `json:"linkedAt"`
}

func setupIdentitiesHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up identity linking endpoint handlers...")

	g.POST("/internal/identities/link/start", linkStartHandler(authzClient))
	g.POST("/internal/identities/link/complete", linkCompleteHandler(authzClient))
	g.GET("/api/identities", listIdentitiesHandler(authzClient))
	g.DELETE("/api/identities/:loginProfileId", unlinkIdentityHandler(authzClient))
}

// Mint a link ticket for the caller's own account.
//
//	 Called by auth-web on behalf of an already-active session, forwarding that session's own
//	 Auth0 access token as the bearer credential - verified against auth-api, same as every other
//	 self-service route in this service, rather than a caller-supplied User.id. This is a
//	 deliberate deviation from design.md's original "shared internal-service-secret" sketch:
//	 add-users-api-provisioning never actually added that mechanism (see this service's own
//	 AGENTS.md - the shared-secret fallback was removed, not extended), and resolving the target
//	 User.id from the caller's own verified subject removes the need for one entirely, matching
//	 POST /internal/identities/provision's existing pattern.
//		@Summary		Mint a link ticket
//		@Description	Mint a short-lived, single-use ticket to link a second identity to the caller's account
//		@Tags			internal
//		@Produce		json
//		@Success		200	{object}	linkStartResponse
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/internal/identities/link/start [post]
func linkStartHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		profile, err := models.FindProfileBySubject(c.Request.Context(), subject)
		if err != nil {
			if err == models.ErrProfileNotFound {
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
				return
			}
			logging.Logger.Error("Failed to resolve caller for link ticket", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to resolve caller"})
			return
		}

		ticket, err := models.MintLinkTicket(c.Request.Context(), profile.UserID)
		if err != nil {
			logging.Logger.Error("Failed to mint link ticket", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "mint_failed", Message: "failed to mint link ticket"})
			return
		}

		c.JSON(http.StatusOK, linkStartResponse{Ticket: ticket.String()})
	}
}

// Complete a link ticket, attaching the caller's verified identity to the ticket's target User.
//
//	 Called by auth-web after the second identity's Auth0 round trip completes. The bearer token
//	 verifies the identity being linked (auth-api's /authz/check, same as every other self-service
//	 route); the ticket - minted by link/start against the original session - carries the target
//	 User.id, so the client never gets to assert which account to link into.
//		@Summary		Complete a link ticket
//		@Description	Attach the caller's verified identity to the ticket's target User
//		@Tags			internal
//		@Accept			json
//		@Produce		json
//		@Param			request	body		linkCompleteRequest	true	"Completion request"
//		@Success		200		{object}	linkCompleteResponse
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		409		{object}	apiv.ErrorVO
//		@Failure		410		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/internal/identities/link/complete [post]
func linkCompleteHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		var req linkCompleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}
		ticketID, err := uuid.Parse(req.Ticket)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid ticket"})
			return
		}

		result, err := models.LinkIdentity(c.Request.Context(), subject, ticketID)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrTicketNotFound):
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "ticket_not_found", Message: "link ticket not found"})
			case errors.Is(err, models.ErrTicketExpired):
				c.JSON(http.StatusGone, apiv.ErrorVO{Error: "ticket_expired", Message: "link ticket expired"})
			case errors.Is(err, models.ErrTicketConsumed):
				c.JSON(http.StatusGone, apiv.ErrorVO{Error: "ticket_consumed", Message: "link ticket already used"})
			case errors.Is(err, models.ErrLinkConflict):
				c.JSON(http.StatusConflict, apiv.ErrorVO{Error: "identity_conflict", Message: "this identity is already linked to a different account"})
			default:
				logging.Logger.Error("Failed to complete identity link", "error", err.Error())
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "link_failed", Message: "failed to link identity"})
			}
			return
		}

		c.JSON(http.StatusOK, linkCompleteResponse{UserID: result.UserID.String(), Created: result.Created})
	}
}

// List the caller's linked login methods.
//
//	 Resolved via the caller's own verified Auth0 subject.
//		@Summary		List linked identities
//		@Description	List the caller's linked login methods
//		@Tags			identities
//		@Produce		json
//		@Success		200	{array}		linkedIdentityResponse
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/api/identities [get]
func listIdentitiesHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		profile, err := models.FindProfileBySubject(c.Request.Context(), subject)
		if err != nil {
			if err == models.ErrProfileNotFound {
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
				return
			}
			logging.Logger.Error("Failed to resolve caller for identity listing", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to resolve caller"})
			return
		}

		identities, err := models.ListLinkedIdentities(c.Request.Context(), profile.UserID)
		if err != nil {
			logging.Logger.Error("Failed to list linked identities", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list linked identities"})
			return
		}

		out := make([]linkedIdentityResponse, 0, len(identities))
		for _, id := range identities {
			out = append(out, linkedIdentityResponse{
				LoginProfileID: id.LoginProfileID.String(),
				ConnectionType: id.ConnectionType,
				LinkedAt:       id.LinkedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
		c.JSON(http.StatusOK, out)
	}
}

// Unlink one of the caller's login methods.
//
//	 Resolved via the caller's own verified Auth0 subject; a client cannot target any other
//	 user's LoginProfile. Rejects the request if it would leave the caller with zero login
//	 methods.
//		@Summary		Unlink a login method
//		@Description	Unlink one of the caller's linked login methods
//		@Tags			identities
//		@Produce		json
//		@Param			loginProfileId	path	string	true	"Login profile id"
//		@Success		204
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		409	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/api/identities/{loginProfileId} [delete]
func unlinkIdentityHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		loginProfileID, err := uuid.Parse(c.Param("loginProfileId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid login profile id"})
			return
		}

		profile, err := models.FindProfileBySubject(c.Request.Context(), subject)
		if err != nil {
			if err == models.ErrProfileNotFound {
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
				return
			}
			logging.Logger.Error("Failed to resolve caller for unlink", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to resolve caller"})
			return
		}

		if err := models.UnlinkIdentity(c.Request.Context(), profile.UserID, loginProfileID); err != nil {
			switch {
			case errors.Is(err, models.ErrProfileNotFound):
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "login profile not found"})
			case errors.Is(err, models.ErrLastLoginMethod):
				c.JSON(http.StatusConflict, apiv.ErrorVO{Error: "last_login_method", Message: "cannot unlink your only remaining login method"})
			default:
				logging.Logger.Error("Failed to unlink identity", "error", err.Error())
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "unlink_failed", Message: "failed to unlink identity"})
			}
			return
		}

		c.Status(http.StatusNoContent)
	}
}
