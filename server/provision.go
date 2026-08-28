package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

type provisionRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type provisionResponse struct {
	UserID  string `json:"userId"`
	Created bool   `json:"created"`
}

func setupProvisionHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up identity provisioning endpoint handlers...")

	g.POST("/internal/identities/provision", provisionHandler(authzClient))
}

// Provision a User/LoginProfile record for a verified Auth0 identity.
//
//	 Find-or-create path called by auth-web during /auth/callback, after Auth0 token exchange
//	 and the auth-api authz check both succeed. Requires the caller's own Auth0 access token as
//	 a bearer credential - verified against auth-api, same as GET /admin/users, rather than
//	 a shared secret. The provisioned subject is the verified token's own sub, not a
//	 client-supplied value, so a caller can only ever provision its own identity.
//		@Summary		Provision a user identity
//		@Description	Find or create a User/LoginProfile for the caller's own Auth0 identity
//		@Tags			internal
//		@Accept			json
//		@Produce		json
//		@Param			request	body		provisionRequest	true	"Provisioning request"
//		@Success		200		{object}	provisionResponse
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/internal/identities/provision [post]
func provisionHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		var req provisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}

		provisionResult, err := models.FindOrCreateUser(c.Request.Context(), subject, req.Name, req.Email)
		if err != nil {
			logging.Logger.Error("Failed to provision user", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "provisioning_failed", Message: "failed to provision user"})
			return
		}

		logging.Logger.Info("provisioned identity",
			"subject", subject, "userId", provisionResult.UserID.String(), "created", provisionResult.Created)
		c.JSON(http.StatusOK, provisionResponse{UserID: provisionResult.UserID.String(), Created: provisionResult.Created})
	}
}
