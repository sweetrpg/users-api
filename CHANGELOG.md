# Changelog

## [Unreleased]

- Repo scaffolding: CI, dependabot, branch protection, community docs (AGENTS.md/CLAUDE.md,
  CONTRIBUTING.md, SECURITY.md, CODE_OF_CONDUCT.md).
- `RolesController`'s `/api/admin/*` routes now accept an `X-Internal-Service-Token` shared
  secret as an alternative to an Auth0 bearer token, so `admin-web` (which never holds an Auth0
  access token of its own) can call them.
- Every `/api/admin/*` mutation now writes a before/after `AdminActionAuditLog` row (acting
  user's `sub`, action, target user, detail, status) - if the pre-action write itself fails, the
  action is refused rather than performed unlogged. Internal-service-token callers must send
  `X-Acting-User-Sub` identifying the acting admin.
- Added `kubernetes/` deployment manifests (base + dev/local overlays) - this service previously
  had none. Added a `GET /status/ping` route for the liveness/readiness probes to target, since
  none existed. Fixed a latent bug where Redis configuration read `REDIS_HOSTNAME` (never set by
  the Dockerfile or anywhere else) instead of `REDIS_HOST`/`REDIS_PORT`; added `REDIS_DB` support
  and registered this service's own (mostly vestigial) session store on index 3 of the shared
  `redis.sweetrpg-support` instance - see `sweetrpg/platform`'s `docs/frontend-conventions.md`.
