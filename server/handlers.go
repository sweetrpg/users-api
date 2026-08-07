package server

import (
	"github.com/gin-gonic/gin"
)

func SetupHandlers(g *gin.Engine) {
	setupAdminUsersHandlers(g)
	setupStatusHandlers(g)
}
