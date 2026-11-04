package server

import (
	"crypto/subtle"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/users-api/constants"
)

const internalServiceTokenHeader = "X-Internal-Service-Token"

// hasValidInternalServiceToken reports whether the request carries the shared service token,
// compared in constant time. Always false when INTERNAL_SERVICE_TOKEN is unset, so an
// unconfigured deployment never trusts an empty header.
func hasValidInternalServiceToken(c *gin.Context) bool {
	want := os.Getenv(constants.INTERNAL_SERVICE_TOKEN)
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.GetHeader(internalServiceTokenHeader)), []byte(want)) == 1
}
