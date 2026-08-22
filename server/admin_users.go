package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/constants"
	"github.com/sweetrpg/users-api/models"
)

const (
	bearerPrefix = "Bearer "
)

func setupAdminUsersHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up admin users endpoint handlers...")

	g.GET("/api/admin/users", listUsersHandler(authzClient))
}

func bearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, bearerPrefix) {
		return ""
	}
	return strings.TrimPrefix(auth, bearerPrefix)
}

// List user identities.
//
//	 Minimal id/email/subject listing for admin-web's role/service-access management UI to
//	 compose against auth-api's role/deny-entry data. Requires a forwarded user bearer token
//	 carrying the admin role.
//		@Summary		List user identities
//		@Description	List user identities
//		@Tags			admin
//		@Produce		json
//		@Success		200		{array}		models.UserIdentity
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		403		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/api/admin/users [get]
func listUsersHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
			return
		}

		result, err := authzClient.Check(c.Request.Context(), token, constants.ServiceName)
		if err != nil {
			if _, ok := err.(authz.InvalidTokenError); ok {
				c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
				return
			}
			logging.Logger.Error("authz check failed", "error", err.Error())
			c.JSON(http.StatusServiceUnavailable, apiv.ErrorVO{Error: "authz_unavailable", Message: "Unable to verify authorization"})
			return
		}
		if !result.Allowed || !authz.HasRole(result.Roles, authz.RoleAdmin) {
			c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "caller does not have a qualifying role"})
			return
		}

		identities, err := models.ListUserIdentities(c.Request.Context())
		if err != nil {
			logging.Logger.Error("Failed to list user identities", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to list users"})
			return
		}

		c.JSON(http.StatusOK, identities)
	}
}
