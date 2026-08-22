## [0.6.3] - 2026-08-22

### 🐛 Bug Fixes

- *(k8s)* Drop dangling secrets/misc.yaml reference from dev overlay

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.6.2
## [0.6.2] - 2026-08-22

### 🚜 Refactor

- *(auth)* Remove legacy X-Internal-Service-Token fallback from admin users listing

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.6.1
## [0.6.1] - 2026-08-21

### 🐛 Bug Fixes

- *(kubernetes)* Fix cpu resource limit quantity that never matched ArgoCD's applied manifest

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.6.0
## [0.6.0] - 2026-08-18

### 🚀 Features

- *(local)* Add MongoDB database configuration to configmap
- *(auth)* Authorize admin users listing on forwarded user token, not shared secret

### 📚 Documentation

- Describe the Go service instead of Vapor in README.md

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.5.0
- Remove Swift/Vapor source tree, no longer used
- Reloader, pod monitor
- *(kubernetes)* Wire AUTH_API_URL in dev overlay
## [0.5.0] - 2026-08-11

### 🚀 Features

- Gate continuous profiling behind the profiling-enabled feature flag

### 🐛 Bug Fixes

- *(ci)* Scope Docker Build's concurrency group by ref

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.4.1
- Remove flags from repo
- Annotations for feature flags
- Bump api-core.go to v0.1.0
## [0.4.1] - 2026-08-07

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.4.0
## [0.4.0] - 2026-08-07

### 🚀 Features

- Scaffold Go rewrite of users-api

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.3.4
- Clean up the files
- Remove image patch
- Switch users-api CI/build toolchain from Swift to Go
- Point users-api Kubernetes manifests at the Go build
- Remove duplicate secret
## [0.3.4] - 2026-08-07

### 🐛 Bug Fixes

- *(kubernetes)* Scope users-api Atlas role to its own database
- *(kubernetes)* Scope AtlasDatabaseUser role to sweetrpg-users
- *(kubernetes)* Authenticate AtlasDatabaseUser against admin, not app db

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.3.3
## [0.3.3] - 2026-08-07

### 🐛 Bug Fixes

- *(dependencies)* Upgrade fluent-mongo-driver to 1.1.0+

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.3.2
## [0.3.2] - 2026-08-07

### 🐛 Bug Fixes

- Secret setup for Atlas
- *(kubernetes)* Build MongoDB connection string from component parts

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.3.1
## [0.3.1] - 2026-08-07

### 🐛 Bug Fixes

- Db secret path

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.3.0
## [0.3.0] - 2026-08-06

### 🚀 Features

- Add ingress for service in local
- Move authz to auth-api; add minimal admin user listing

### 🐛 Bug Fixes

- Update ingress setup
- *(kubernetes)* Mount redis-auth secret so REDIS_PASS reaches the app
- *(ci)* Correct sweetrpg/kubernetes manifest path for update-deployment
- Database name and add missing DB vars
- Include Auth0 subject in admin user identity listing
- *(kubernetes)* Version-prefixed Ingress paths with a latest-alias Service

### ⚙️ Miscellaneous Tasks

- *(release)* Merge master into develop after v0.2.0
- *(kubernetes)* Rename namespace sweetrpg-user -> sweetrpg-users, own Redis
- Update middleware name
- Add pod monitor
- Update ingress name
- Add Atlas DB user manifest
## [0.2.0] - 2026-08-02

### 🚀 Features

- Verify Auth0 access tokens against a cached JWKS (#4)
- Add UserRole/ServiceDenyEntry migrations and user provisioning (#5)
- Implement POST /authz/check (#6)
- *(users-api)* Add admin role and access management endpoints
- Accept an internal service token on the admin role/service-access routes
- *(users-api)* Fail-closed audit logging for admin role/access mutations
- *(users-api)* Add kubernetes deployment manifests
- *(users-api)* Add guarded Sentry error reporting

### 🐛 Bug Fixes

- *(ci)* Pin release workflow's Swift toolchain to 6.2
- *(kubernetes)* Point local overlay at the shared local MongoDB
- *(docker)* Cap compiler parallelism to avoid CI OOM
- *(docker)* Copy the actual built binary, Run not App

### ⚙️ Miscellaneous Tasks

- Ignore vapor/jwt major version bumps in dependabot (#7)
- *(release)* Merge master into develop after v0.1.0 (#12)
- *(users-api)* Build amd64/arm64 natively in parallel, cache layers
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
