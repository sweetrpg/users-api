# AGENTS.md

This file provides guidance to Claude Code, Codex, GitHub Copilot, and other coding agents
working in this repository.

## About This Project

`users-api` (Go) is the platform's user profile/management service. Renamed from `profiles-api`
(see `sweetrpg/platform`'s `add-user-api-authn-authz` OpenSpec change) to reflect that expanded
scope. Rewritten from Swift/Vapor to Go - see `sweetrpg/platform`'s `migrate-auth-users-api-to-go`
OpenSpec change for the rationale (closing an observability gap: no tracing, no structured
logging, no trace-header propagation) and migration strategy (replaced the Swift deployment in
place - no parallel `api-v0`/`api-v1` deploy; the platform is pre-MVP with no versioned-API
contract to protect yet, see design.md's "Cutover strategy" decision). It reads/writes the same
MongoDB collections
(`users`, `login_profiles`) the Swift service produced; no data migration was needed. This repo's
`users-model` Swift package dependency was dropped along with it.

Authentication and authorization (Auth0 JWKS token verification, the role model, per-service
access, `POST /authz/check`) used to live here but moved to a dedicated `auth-api` service - see
`sweetrpg/platform`'s `split-authz-into-auth-api` OpenSpec change for the rationale. `users-api`
now holds only profile/management data - in practice, only the single admin listing route below;
the Swift service's `UsersController`/`AuthController` (user registration, OAuth login) were
already dead code (zero routes registered) by the time of the Go rewrite and were not ported.

### Consumers

- `admin-web`: calls `GET /api/admin/users` (`AdminUsersController`) for a minimal id/email
  listing, which it composes with `auth-api`'s role/deny-entry data to build its
  role/service-access management UI - `users-api` itself has no authorization data to serve.
- `users-web`: end-user profile/settings pages.

### Caller auth (`admin-web` → `AdminUsersController`)

`admin_users.go`'s `GET /api/admin/users` route requires a forwarded user bearer token carrying
the `admin` role, verified against `auth-api`'s `/authz/check` via the `authz` package - see
`sweetrpg/platform`'s `api-client-auth` OpenSpec change. `admin-web` forwards the acting admin's
own Auth0 access token from its shared session as the bearer credential. The former shared-secret
fallback (`X-Internal-Service-Token`) was removed once all known callers had migrated.

## Self-service profile routes

All use the caller's own bearer token, verified by `auth-api`; the user is whoever the token
resolves to and no user ID is read from a request.

| Route | Purpose |
| --- | --- |
| `GET /profile` | The caller's profile, including `terms_accepted` (`{version, accepted_at}`, or `null`) |
| `PATCH /profile` | Update name, bio, website, username |
| `PUT /profile/terms` | Record acceptance of a terms version: body `{"version": "1.0"}` |

`PUT /profile/terms` returns `{"version", "accepted_at"}` (200). `version` is dotted numeric (`1.0`,
`1.0.1`; one to four segments, no leading zeros) and compares segment by segment as numbers
(`1.10` is newer than `1.9`; `1.0` equals `1.0.0`). `accepted_at` is set by the server, in UTC.
Accepting the same version again returns the stored record unchanged (the original `accepted_at`).
A version that is not newer than the stored one is `409` `terms_version_not_newer` with
`current_version`. A malformed version is `400` `invalid_version`; no profile yet is `404`. The
record changes only by accepting a newer version; there is no admin edit and no clear. The write is
a compare-and-set on the stored version, so two racing acceptances cannot lower the version, and
the document's `updated_at`/`updated_by` are stamped with the acceptance time and the owner.

## Language and Framework

Go, following `sweetrpg/platform`'s `docs/service-conventions.md` baseline: Gin, `api-core.go`
(tracing setup, `/status/health`/`/status/ping`), `mongodb.go` (generic Mongo CRUD, connection
lifecycle), `common.go` (structured application logging), `slog-gin` (JSON HTTP access logs).
Handlers live in `server/` (one file per resource), models in `models/`, env-var/collection-name
constants in `constants/`, the entrypoint in `cmd/users-api/main.go`. Swagger docs (`docs/`) are
generated, not hand-written - see "Running Checks Locally".

The Go rewrite dropped the Swift service's Redis-backed Vapor session store (it existed only for
the already-dead OAuth login controllers). Redis is back for one purpose only: the per-client/IP
rate limiter's distributed counters (see "Rate limiting"), not sessions.

## Internal calls

`POST /internal/resolve-subjects` maps Auth0 subjects to canonical user IDs for callers that hold an
admin user's bearer token (auth-api's admin routes, the backfill commands). There are no service
tokens (PADR-0037). `GET /admin/users` returns `subjects` (every linked login) next to the legacy
`subject` (the first).

## Rate limiting

Per-client/IP rate limiting is on by default via the shared `api-core.go/ratelimit` middleware
(Redis-backed counters keyed by `X-API-Key` else client IP, `cheap` tier for `/status/*`,
fail-closed 503 when Redis is unreachable, 429 on exceed). This replaced the process-wide
`rate.NewLimiter` bucket. `REDIS_HOST`/`REDIS_PORT` are in the dev configmap; `REDIS_PASS` comes
from the `api-cache` `ExternalSecret`. Tune with `RATE_LIMIT_CHEAP`/
`RATE_LIMIT_CHEAP_WINDOW_SECONDS`/`RATE_LIMIT_STANDARD`/`RATE_LIMIT_STANDARD_WINDOW_SECONDS`. See
`platform`'s `openspec/changes/fix-rate-limiting-per-client-ip`.

## Deployment

`kubernetes/` (base + `overlays/{dev,local}`) deploys this service into the `sweetrpg-users`
namespace as a single `api` Deployment/Service - the Go rewrite replaced the Swift image and
manifests in place, not as a parallel version. Rollback is `git revert` the cutover commit and
let ArgoCD resync back to the Swift image. `DB_URI` (or the
`DB_SCHEME`/`DB_HOST`/`DB_USER`/`DB_PW`/`DB_NAME`/`DB_OPTS` parts) come from Akeyless via
`ExternalSecret`s, not the configmap.

## Committing Code

[Conventional Commits](https://www.conventionalcommits.org/): `<type>(<scope>): <description>`.

## Branches and Workflow

Git-flow (see `docs/git-flow.md` in `sweetrpg/platform`): `develop` is the integration branch,
`master` reflects the latest release. Feature/fix branches off `develop`, PR back into `develop`.

## Running Checks Locally

```bash
go build ./...
go vet ./...
go test ./...
```

`go run cmd/users-api/main.go` serves on `:8000` (`BIND_ADDRESS` to override). Regenerate Swagger
docs after changing handler annotations:

```bash
go run github.com/swaggo/swag/cmd/swag@latest init -d cmd/users-api/,server/,models/ --parseDependency --parseInternal
```
