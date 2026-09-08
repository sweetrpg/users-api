package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

// activeUserWindow is the rolling window a login must fall within for a user to count as active,
// per the users-api-admin-stats spec.
const activeUserWindow = 30 * 24 * time.Hour

// newUserWindow is the rolling window a user's creation must fall within to count as new.
const newUserWindow = 7 * 24 * time.Hour

// adminStatsResponse is a plain JSON object, not a JSON:API resource - a singleton aggregate,
// matching catalog-api's /stats shape.
type adminStatsResponse struct {
	TotalUsers  int64 `json:"total_users"`
	ActiveUsers int64 `json:"active_users"`
	NewUsers    int64 `json:"new_users"`
}

// Get user population stats.
//
//	 Total, active, and new user counts for admin-web's platform metrics page. Active = a login
//	 within the last 30 days; new = created within the last 7 days. Requires a forwarded user
//	 bearer token carrying the admin role, same auth model as GET /admin/users.
//		@Summary		Get user population stats
//		@Description	Total user count, active user count (login within a rolling 30-day window), and new user count (created within a rolling 7-day window)
//		@Tags			admin
//		@Produce		json
//		@Success		200		{object}	adminStatsResponse
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		403		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Failure		503		{object}	apiv.ErrorVO
//		@Router			/admin/stats [get]
func adminStatsHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := resolveAdminSubject(c, authzClient); !ok {
			return
		}

		ctx := c.Request.Context()
		total, err := models.CountUsers(ctx)
		if err != nil {
			logging.Logger.Error("Failed to count users", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to count users"})
			return
		}
		active, err := models.CountActiveUsers(ctx, activeUserWindow)
		if err != nil {
			logging.Logger.Error("Failed to count active users", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to count active users"})
			return
		}
		newUsers, err := models.CountNewUsers(ctx, newUserWindow)
		if err != nil {
			logging.Logger.Error("Failed to count new users", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to count new users"})
			return
		}

		c.JSON(http.StatusOK, adminStatsResponse{TotalUsers: total, ActiveUsers: active, NewUsers: newUsers})
	}
}
