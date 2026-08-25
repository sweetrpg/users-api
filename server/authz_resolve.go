package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/constants"
)

// resolveVerifiedSubject forwards the caller's bearer token to auth-api and returns the
// verified Auth0 subject, with no role requirement - any authenticated identity may act on its
// own data. Writes the appropriate error response and returns ok=false if resolution fails, so
// callers can simply `if !ok { return }`.
func resolveVerifiedSubject(c *gin.Context, authzClient *authz.Client) (subject string, ok bool) {
	token := bearerToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
		return "", false
	}

	result, err := authzClient.Check(c.Request.Context(), token, constants.ServiceName)
	if err != nil {
		if _, ok := err.(authz.InvalidTokenError); ok {
			c.JSON(http.StatusUnauthorized, apiv.ErrorVO{Error: "unauthorized", Message: "missing or invalid credentials"})
			return "", false
		}
		logging.Logger.Error("authz check failed", "error", err.Error())
		c.JSON(http.StatusServiceUnavailable, apiv.ErrorVO{Error: "authz_unavailable", Message: "Unable to verify authorization"})
		return "", false
	}
	if !result.Allowed || result.Sub == "" {
		c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "caller is not authorized"})
		return "", false
	}

	return result.Sub, true
}
