package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/users-api/authz"
)

func SetupHandlers(g *gin.Engine, authzClient *authz.Client) {
	setupAdminUsersHandlers(g, authzClient)
	setupStatusHandlers(g)
}
