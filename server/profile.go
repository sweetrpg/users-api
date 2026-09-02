package server

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/users-api/authz"
	"github.com/sweetrpg/users-api/models"
)

// bioMaxLength bounds bio to a reasonable profile-blurb size - see design.md's "fixed max
// length" decision; the exact number is cosmetic, not part of the API contract.
const bioMaxLength = 500

type profileResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Bio      string    `json:"bio"`
	Website  string    `json:"website"`
	Username string    `json:"username"`
}

type updateProfileRequest struct {
	Name     string `json:"name"`
	Bio      string `json:"bio"`
	Website  string `json:"website"`
	Username string `json:"username"`
}

func setupProfileHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up self-service profile endpoint handlers...")

	g.GET("/profile", getProfileHandler(authzClient))
	g.PATCH("/profile", updateProfileHandler(authzClient))
}

// Get the caller's own profile.
//
//	 Resolved via the caller's own verified Auth0 subject - never a path or query parameter, so
//	 this route can only ever return the caller's own data.
//		@Summary		Get the caller's own profile
//		@Description	Get the caller's own profile
//		@Tags			profile
//		@Produce		json
//		@Success		200	{object}	profileResponse
//		@Failure		401	{object}	apiv.ErrorVO
//		@Failure		404	{object}	apiv.ErrorVO
//		@Failure		500	{object}	apiv.ErrorVO
//		@Router			/profile [get]
func getProfileHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		profile, err := models.FindProfileBySubject(c.Request.Context(), subject)
		if err != nil {
			if err == models.ErrProfileNotFound {
				logging.Logger.Warn("profile lookup found no User/LoginProfile for subject", "subject", subject)
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
				return
			}
			logging.Logger.Error("Failed to fetch profile", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to fetch profile"})
			return
		}

		c.JSON(http.StatusOK, profileResponse{
			UserID: profile.UserID, Name: profile.Name, Email: profile.Email,
			Bio: profile.Bio, Website: profile.Website, Username: profile.Username,
		})
	}
}

// Update the caller's own profile.
//
//	 Updates name/bio/website/username - email is read-only, sourced from Auth0 at provisioning
//	 time (see design.md). An empty username in the body leaves the existing one untouched.
//	 Resolved via the caller's own verified Auth0 subject; a client cannot target any other
//	 user's record.
//		@Summary		Update the caller's own profile
//		@Description	Update the caller's own profile
//		@Tags			profile
//		@Accept			json
//		@Produce		json
//		@Param			request	body		updateProfileRequest	true	"Profile update"
//		@Success		200		{object}	profileResponse
//		@Failure		400		{object}	apiv.ErrorVO
//		@Failure		401		{object}	apiv.ErrorVO
//		@Failure		404		{object}	apiv.ErrorVO
//		@Failure		409		{object}	apiv.ErrorVO
//		@Failure		500		{object}	apiv.ErrorVO
//		@Router			/profile [patch]
func updateProfileHandler(authzClient *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := resolveVerifiedSubject(c, authzClient)
		if !ok {
			return
		}

		var req updateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "invalid request body"})
			return
		}
		if msg := validateProfileUpdate(req); msg != "" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: msg})
			return
		}

		existing, err := models.FindProfileBySubject(c.Request.Context(), subject)
		if err != nil {
			if err == models.ErrProfileNotFound {
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
				return
			}
			logging.Logger.Error("Failed to fetch profile", "error", err.Error())
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: "failed to fetch profile"})
			return
		}

		// An empty username in the body means "leave it as it is" - don't wipe a backfilled
		// username just because the client didn't send one.
		username := existing.Username
		if req.Username != "" {
			username = req.Username
		}

		if err := models.UpdateProfile(c.Request.Context(), existing.UserID, existing.UserID.String(), req.Name, req.Bio, req.Website, username); err != nil {
			switch err {
			case models.ErrProfileNotFound:
				c.JSON(http.StatusNotFound, apiv.ErrorVO{Error: "not_found", Message: "no profile for this identity yet"})
			case models.ErrUsernameTaken:
				c.JSON(http.StatusConflict, apiv.ErrorVO{Error: "username_taken", Message: "that username is already taken"})
			default:
				logging.Logger.Error("Failed to update profile", "error", err.Error())
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: "failed to update profile"})
			}
			return
		}

		c.JSON(http.StatusOK, profileResponse{
			UserID: existing.UserID, Name: req.Name, Email: existing.Email, Bio: req.Bio, Website: req.Website, Username: username,
		})
	}
}

// validateProfileUpdate returns a user-facing message if req is invalid, or "" if valid. An
// empty username is allowed (means "leave unchanged"); a non-empty one must match the accepted
// format.
func validateProfileUpdate(req updateProfileRequest) string {
	if len(req.Bio) > bioMaxLength {
		return "bio must be 500 characters or fewer"
	}
	if req.Website != "" {
		parsed, err := url.Parse(req.Website)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "website must be a valid http(s):// URL"
		}
	}
	if req.Username != "" && !models.ValidUsername(req.Username) {
		return "username must be 3-30 characters of lowercase letters, digits, hyphen, or underscore"
	}
	return ""
}
