package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

type acceptTermsRequest struct {
	// Version is the terms version being accepted: dotted numeric, such as 1.0 or 1.0.1.
	Version string `json:"version"`
}

// termsNotNewerResponse is the 409 body: the error and the version already on record.
type termsNotNewerResponse struct {
	Error          string `json:"error"`
	Message        string `json:"message"`
	CurrentVersion string `json:"current_version"`
}

// Accept the terms of service.
//
//	 Records the caller's acceptance of a terms version on their own profile. The user is the one
//	 the verified token resolves to; no user ID is read from the request, so a caller can only
//	 accept for themselves. accepted_at is set by the server in UTC. Accepting the same version
//	 again returns the stored record unchanged (200); a version that is not newer than the stored
//	 one is refused with 409 terms_version_not_newer. There is no way to edit or clear an
//	 acceptance except by accepting a newer version.
//		@Summary		Accept the terms of service
//		@Description	Record the caller's acceptance of a terms version
//		@Tags			profile
//		@Accept			json
//		@Produce		json
//		@Param			request	body		acceptTermsRequest	true	"Version being accepted"
//		@Success		200		{object}	models.TermsAccepted
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		404		{object}	apiv.ErrorVO
//		@Failure		409		{object}	termsNotNewerResponse
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/profile/terms [put]
func acceptTermsHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		var req acceptTermsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}
		if !models.ValidTermsVersion(req.Version) {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_version", Message: "version must be dotted numeric, like 1.0 or 1.0.1"})
			return
		}

		profile, err := profileLookup(c.Request.Context(), subject)
		if err != nil {
			respondTermsError(c, err)
			return
		}

		accepted, err := termsStore.Accept(c.Request.Context(), profile.UserID, req.Version, time.Now().UTC())
		if err != nil {
			respondTermsError(c, err)
			return
		}
		c.JSON(http.StatusOK, accepted)
	}
}

func respondTermsError(c *gin.Context, err error) {
	var notNewer *models.TermsNotNewerError
	switch {
	case errors.As(err, &notNewer):
		c.JSON(http.StatusConflict, termsNotNewerResponse{
			Error: "terms_version_not_newer", Message: "that version is not newer than the one already accepted", CurrentVersion: notNewer.Current,
		})
	case errors.Is(err, models.ErrProfileNotFound):
		c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
	default:
		logging.Logger.Error("Failed to record terms acceptance", "error", err.Error())
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to record acceptance"})
	}
}
