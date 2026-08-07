package server

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
	"github.com/sweetrpg/users-api/constants"
	"github.com/sweetrpg/users-api/models"
)

const internalServiceTokenHeader = "X-Internal-Service-Token"

func setupAdminUsersHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up admin users endpoint handlers...")

	g.GET("/api/admin/users", listUsers)
}

// hasValidInternalServiceToken reports whether the request presented the
// correct X-Internal-Service-Token header, compared constant-time against
// INTERNAL_SERVICE_TOKEN. Always false when INTERNAL_SERVICE_TOKEN is unset -
// the internal-auth path is then permanently disabled rather than trusting an
// empty token, matching the Swift service's InternalServiceAuth contract.
// Unlike admin-api's WriteAuth, this route is read-only and not audited, so
// it does not require an X-Acting-User-Sub header.
func hasValidInternalServiceToken(c *gin.Context) bool {
	expected := util.GetEnv(constants.INTERNAL_SERVICE_TOKEN, "")
	if expected == "" {
		return false
	}
	presented := c.GetHeader(internalServiceTokenHeader)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) == 1
}

// List user identities.
//
//	 Minimal id/email/subject listing for admin-web's role/service-access management UI to compose against auth-api's role/deny-entry data. Requires the internal service token.
//		@Summary		List user identities
//		@Description	List user identities
//		@Tags			admin
//		@Produce		json
//		@Param			X-Internal-Service-Token	header	string	true	"Shared internal-service secret"
//		@Success		200		{array}		models.UserIdentity
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/api/admin/users [get]
func listUsers(c *gin.Context) {
	if !hasValidInternalServiceToken(c) {
		c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid internal service credentials"})
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
