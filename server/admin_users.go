package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/constants"
	"github.com/sweetrpg/users-api/models"
)

const (
	internalServiceTokenHeader = "X-Internal-Service-Token"
	bearerPrefix               = "Bearer "
)

func setupAdminUsersHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up admin users endpoint handlers...")

	g.GET("/api/admin/users", listUsersHandler(authzClient))
}

// hasValidInternalServiceToken reports whether the request presented the correct
// X-Internal-Service-Token header, compared constant-time against INTERNAL_SERVICE_TOKEN.
// Always false when INTERNAL_SERVICE_TOKEN is unset - the legacy path is then permanently
// disabled rather than trusting an empty token.
func hasValidInternalServiceToken(c *gin.Context) bool {
	expected := util.GetEnv(constants.INTERNAL_SERVICE_TOKEN, "")
	if expected == "" {
		return false
	}
	presented := c.GetHeader(internalServiceTokenHeader)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) == 1
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
//	 compose against auth-api's role/deny-entry data. Requires either a forwarded user bearer
//	 token carrying the admin role, or (legacy, during migration) the internal service token.
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
		if token := bearerToken(c); token != "" {
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
		} else if !hasValidInternalServiceToken(c) {
			c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
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
