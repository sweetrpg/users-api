package server

import (
	"net/http"
	"strconv"
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

const (
	statsHistoryDefaultDays = 30
	statsHistoryMinDays     = 1
	statsHistoryMaxDays     = 90
)

// adminStatsHistoryEntry is one UTC calendar day's cumulative and same-day user counts.
type adminStatsHistoryEntry struct {
	Date       string `json:"date"`
	TotalUsers int64  `json:"total_users"`
	NewUsers   int64  `json:"new_users"`
}

// Get a daily user-growth history.
//
//	 A per-UTC-calendar-day series ending today, oldest first, for admin-web's metrics chart.
//	 new_users is that day's signups; total_users is the cumulative non-soft-deleted user count
//	 as of the end of that day. No active series - last_login_at only holds the latest login, so
//	 historical daily-active can't be reconstructed. Same admin gate as GET /admin/stats.
//		@Summary		Daily user-growth history
//		@Description	Per-day total and new user counts, oldest first, last element is today
//		@Tags			admin
//		@Produce		json
//		@Param			days	query		int	false	"Number of days back to include (default 30, clamped to [1, 90])"
//		@Success		200		{array}		adminStatsHistoryEntry
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		403		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Failure		503		{object}	apiv.ErrorVO
//		@Router			/admin/stats/history [get]
func adminStatsHistoryHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := resolveAdminSubject(c, authzClient); !ok {
			return
		}

		days := statsHistoryDefaultDays
		if raw := c.Query("days"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "days must be an integer"})
				return
			}
			days = n
		}
		if days < statsHistoryMinDays {
			days = statsHistoryMinDays
		}
		if days > statsHistoryMaxDays {
			days = statsHistoryMaxDays
		}

		ctx := c.Request.Context()
		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		windowStart := today.AddDate(0, 0, -(days - 1))
		windowEnd := today.AddDate(0, 0, 1) // exclusive: tomorrow 00:00Z

		daily, err := models.DailyNewUsers(ctx, windowStart, windowEnd)
		if err != nil {
			logging.Logger.Error("Failed to aggregate daily new users", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to build stats history"})
			return
		}
		cumulative, err := models.CountUsersCreatedBefore(ctx, windowStart)
		if err != nil {
			logging.Logger.Error("Failed to count baseline users", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to build stats history"})
			return
		}

		out := make([]adminStatsHistoryEntry, 0, days)
		for i := 0; i < days; i++ {
			key := windowStart.AddDate(0, 0, i).Format("2006-01-02")
			n := daily[key]
			cumulative += n
			out = append(out, adminStatsHistoryEntry{Date: key, TotalUsers: cumulative, NewUsers: n})
		}
		c.JSON(http.StatusOK, out)
	}
}
