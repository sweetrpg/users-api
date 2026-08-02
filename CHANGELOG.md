## [0.2.0] - 2026-08-02

### 🚀 Features

- Verify Auth0 access tokens against a cached JWKS (#4)
- Add UserRole/ServiceDenyEntry migrations and user provisioning (#5)
- Implement POST /authz/check (#6)
- *(users-api)* Add admin role and access management endpoints
- Accept an internal service token on the admin role/service-access routes
- *(users-api)* Fail-closed audit logging for admin role/access mutations
- *(users-api)* Add kubernetes deployment manifests

### 🐛 Bug Fixes

- *(ci)* Pin release workflow's Swift toolchain to 6.2
- *(kubernetes)* Point local overlay at the shared local MongoDB
- *(docker)* Cap compiler parallelism to avoid CI OOM
- *(docker)* Copy the actual built binary, Run not App

### ⚙️ Miscellaneous Tasks

- Ignore vapor/jwt major version bumps in dependabot (#7)
- *(release)* Merge master into develop after v0.1.0 (#12)
# Changelog

## [Unreleased]

- Added `kubernetes/` deployment manifests (base + dev/local overlays) - this service previously
  had none. Added a `GET /status/ping` route for the liveness/readiness probes to target, since
  none existed. Fixed a latent bug where Redis configuration read `REDIS_HOSTNAME` (never set by
  the Dockerfile or anywhere else) instead of `REDIS_HOST`/`REDIS_PORT`; added `REDIS_DB` support
  and registered this service's own (mostly vestigial) session store on index 3 of the shared
  `redis.sweetrpg-support` instance - see `sweetrpg/platform`'s `docs/frontend-conventions.md`.
- Fixed the Release workflow's Swift toolchain pin (`swift-version: "6.2"`), matching ci.yaml/
  pr.yaml, so `swift-nio`/`swift-log` resolve correctly during release builds.

## [0.1.0] - 2026-08-01

### 🚀 Features

- Verify Auth0 access tokens against a cached JWKS (#4)
- Add UserRole/ServiceDenyEntry migrations and user provisioning (#5)
- Implement POST /authz/check (#6)
- *(users-api)* Add admin role and access management endpoints
- Accept an internal service token on the admin role/service-access routes
- *(users-api)* Fail-closed audit logging for admin role/access mutations

### ⚙️ Miscellaneous Tasks

- Ignore vapor/jwt major version bumps in dependabot (#7)
