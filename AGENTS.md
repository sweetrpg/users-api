# AGENTS.md

This file provides guidance to Claude Code, Codex, GitHub Copilot, and other coding agents
working in this repository.

## About This Project

`users-api` (Swift/Vapor) is the platform's user profile/management service. Renamed from
`profiles-api` (see `sweetrpg/platform`'s `add-user-api-authn-authz` OpenSpec change) to reflect
that expanded scope, and paired with `users-model` for its shared model layer.

Authentication and authorization (Auth0 JWKS token verification, the role model, per-service
access, `POST /authz/check`) used to live here but moved to a dedicated `auth-api` service - see
`sweetrpg/platform`'s `split-authz-into-auth-api` OpenSpec change for the rationale. `users-api`
now holds only profile/management data.

### Consumers

- `admin-web`: calls `GET /api/admin/users` (`AdminUsersController`) for a minimal id/email
  listing, which it composes with `auth-api`'s role/deny-entry data to build its
  role/service-access management UI - `users-api` itself has no authorization data to serve.
- `users-web`: end-user profile/settings pages.

### Internal service auth (`admin-web` → `AdminUsersController`)

`AdminUsersController`'s `GET /api/admin/users` route accepts `X-Internal-Service-Token`
(matching the `INTERNAL_SERVICE_TOKEN` env var, see `InternalServiceAuth.swift`) as its only
authentication - `admin-web` never holds an Auth0 access token of its own (it reads
`auth-web`'s shared session instead). Whoever holds this shared secret is trusted outright, on
the assumption that `admin-web`'s own `AuthRequiredMiddleware` already verified the acting user
has the `admin` role before ever making the call. This is a narrower purpose than it used to
serve (it used to gate the full role/deny-entry CRUD surface too, before that moved to
`auth-api`) - `INTERNAL_SERVICE_TOKEN` here and `auth-api`'s own copy of the same mechanism are
independent secrets, not required to match.

## Deployment

`kubernetes/` (base + `overlays/{dev,local}`) deploys this service into the `sweetrpg-users`
namespace as `api-v1`. `DATABASE_URL` (MongoDB) and `INTERNAL_SERVICE_TOKEN` come from Akeyless
via `ExternalSecret`s, not the configmap. This service's own dedicated Redis instance
(`redis.sweetrpg-users.svc.cluster.local`) is its own Vapor session store, registered in
`sweetrpg/platform`'s `docs/frontend-conventions.md`, unrelated to `auth-web`'s suite-wide
shared session in `sweetrpg-auth`, which this service never reads or writes.

## Committing Code

[Conventional Commits](https://www.conventionalcommits.org/): `<type>(<scope>): <description>`.

## Branches and Workflow

Git-flow (see `docs/git-flow.md` in `sweetrpg/platform`): `develop` is the integration branch,
`master` reflects the latest release. Feature/fix branches off `develop`, PR back into `develop`.

## Running Checks Locally

```bash
swift build
swift test
swift format lint --recursive --strict Sources Tests
```

`swift run` serves on `:8080`. Without `REDIS_HOST` set, falls back to in-memory sessions and no
response caching.
