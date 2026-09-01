package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/users-api/authz"
)

func SetupHandlers(g *gin.Engine, authzClient *authz.Client) {
	setupAdminUsersHandlers(g, authzClient)
	setupProvisionHandlers(g, authzClient)
	setupProfileHandlers(g, authzClient)
	setupFriendsHandlers(g, authzClient)
	setupStatusHandlers(g)
}
