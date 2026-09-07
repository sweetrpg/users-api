package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

type resolveSubjectsRequest struct {
	Subjects []string `json:"subjects"`
}

func setupResolveSubjectsHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up resolve subjects endpoint handlers...")

	g.POST("/internal/resolve-subjects", resolveSubjectsHandler(authzClient))
}

// Resolve a batch of Auth0 subjects to the canonical users._id each maps to.
//
//	 Internal batch endpoint for the canonical-user-id backfills: maps a batch of `*_by` subject
//	 values to the user ids they belong to, so audit fields written with a pre-canonical prompt
//	 subject can be rewritten to the user's canonical id. Requires an admin bearer token, the
//	 same write-auth the admin routes use - the caller is a trusted platform backfill, not an
//	 end user. Subjects with no resolvable user (no non-deleted Auth0 login profile) are omitted
//	 from the response; the caller treats those as the "system" actor (see the
//	 canonical-user-id-provenance design Decision 3).
//		@Summary		Resolve subjects to user ids
//		@Description	Resolve a batch of Auth0 subjects to the canonical users._id each maps to
//		@Tags			internal
//		@Accept			json
//		@Produce		json
//		@Param			request	body		resolveSubjectsRequest	true	"Subjects to resolve"
//		@Success		200		{object}	map[string]string
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		403		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/internal/resolve-subjects [post]
func resolveSubjectsHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := resolveAdminSubject(c, authzClient); !ok {
			return
		}

		var req resolveSubjectsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}

		resolved, err := models.ResolveSubjectIDs(c.Request.Context(), req.Subjects)
		if err != nil {
			logging.Logger.Error("Failed to resolve subjects", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to resolve subjects"})
			return
		}

		c.JSON(http.StatusOK, resolved)
	}
}
