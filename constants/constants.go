package constants

// Environment variable names
const (
	HEALTH_TOKEN             = "HEALTH_TOKEN"
	ALLOWED_ORIGINS          = "ALLOWED_ORIGINS"
	PYROSCOPE_SERVER_ADDRESS = "PYROSCOPE_SERVER_ADDRESS"
	PYROSCOPE_TENANT_ID      = "PYROSCOPE_TENANT_ID"

	// INTERNAL_SERVICE_TOKEN gates the /api/admin/users route; see
	// server.hasValidInternalServiceToken. Preserves the Swift service's
	// InternalServiceAuth contract exactly.
	INTERNAL_SERVICE_TOKEN = "INTERNAL_SERVICE_TOKEN"
)

// Value constants
const (
	ServiceName = "users-api"

	// UsersCollection is the MongoDB collection name for user profile
	// documents, unchanged from the Swift service's Fluent schema
	// (UserModel's User.v20210620.schemaName).
	UsersCollection = "users"

	// LoginProfilesCollection is the MongoDB collection name for third-party
	// login profile documents, unchanged from the Swift service's Fluent
	// schema (UserModel's LoginProfile.v20210620.schemaName).
	LoginProfilesCollection = "login_profiles"

	// Auth0ThirdPartyAuth is the LoginProfile.thirdPartyAuth value identifying
	// an Auth0-issued login profile - the only third-party auth provider the
	// platform uses today.
	Auth0ThirdPartyAuth = "auth0"
)
