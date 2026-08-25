package server

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/models"
)

const internalServiceTokenHeader = "X-Internal-Service-Token"

type provisionRequest struct {
	Subject string `json:"subject" binding:"required"`
	Name    string `json:"name"`
	Email   string `json:"email"`
}

type provisionResponse struct {
	UserID  string `json:"userId"`
	Created bool   `json:"created"`
}

func setupProvisionHandlers(g *gin.Engine, internalServiceToken string) {
	logging.Logger.Info("Setting up identity provisioning endpoint handlers...")

	g.POST("/internal/identities/provision", requireInternalServiceToken(internalServiceToken), provisionHandler())
}

// requireInternalServiceToken gates a route on a shared-secret header, compared in constant
// time. A blank configuredToken rejects every request rather than trusting an empty header.
func requireInternalServiceToken(configuredToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader(internalServiceTokenHeader)
		if configuredToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(configuredToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
			return
		}
		c.Next()
	}
}

// Provision a User/LoginProfile record for a verified Auth0 identity.
//
//	 Find-or-create path called by auth-web during /auth/callback, after Auth0 token exchange
//	 and the auth-api authz check both succeed. Requires the internal service token header.
//		@Summary		Provision a user identity
//		@Description	Find or create a User/LoginProfile for an Auth0 subject
//		@Tags			internal
//		@Accept			json
//		@Produce		json
//		@Param			request	body		provisionRequest	true	"Provisioning request"
//		@Success		200		{object}	provisionResponse
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/internal/identities/provision [post]
func provisionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req provisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "subject is required"})
			return
		}

		result, err := models.FindOrCreateUser(c.Request.Context(), req.Subject, req.Name, req.Email)
		if err != nil {
			logging.Logger.Error("Failed to provision user", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "provisioning_failed", Message: "failed to provision user"})
			return
		}

		c.JSON(http.StatusOK, provisionResponse{UserID: result.UserID.String(), Created: result.Created})
	}
}
