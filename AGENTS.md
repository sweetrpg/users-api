# AGENTS.md

This file provides guidance to Claude Code, Codex, GitHub Copilot, and other coding agents
working in this repository.

## About This Project

`users-api` (Swift/Vapor) is the platform's identity service: authentication, authorization,
profile data, settings, and entitlements. Renamed from `profiles-api` (see
`sweetrpg/platform`'s `add-user-api-authn-authz` OpenSpec change) to reflect that expanded
scope, and paired with `users-model` for its shared model layer.

It verifies Auth0-issued access tokens server-side (JWKS signature verification, not a local
unverified decode) and exposes `POST /authz/check` for any other service to call with a bearer
token plus a service/action pair, getting back an allow/deny decision and the caller's roles.
Role model: `user`, `submitter`, `editor`, `moderator`, `approver`, `admin`. Access is
default-allow per service, with an explicit per-user, per-service deny-list for the `admin`
role to restrict a specific user.

See `sweetrpg/platform`'s `openspec/changes/add-user-api-authn-authz/design.md` for the full
rationale (why default-allow + deny-list, why a fixed role enum, why the rename happened before
any real consumer existed).

### Consumers

- `auth-web`: calls `/authz/check` once at login (bearer token from Auth0's token endpoint) to
  establish a session's verified roles - the platform's sole caller of this endpoint with a real
  end-user Auth0 token. See `sweetrpg/platform`'s `add-user-api-authn-authz` change for why login
  itself lives in `auth-web`, not here.
- `admin-web`: the admin-gated role/service-access management UI (`RolesController`'s
  `/api/admin/*` routes) - see "Internal service auth" below for how it authenticates, since it
  never holds an Auth0 token of its own.
- `users-web`: end-user profile/settings pages only, not the admin UI (that moved to `admin-web`
  partway through the `add-user-api-authn-authz` change - an earlier design had it here).

### Internal service auth (`admin-web` → `RolesController`)

`RolesController`'s `/api/admin/*` routes accept `X-Internal-Service-Token` (matching the
`INTERNAL_SERVICE_TOKEN` env var, see `InternalServiceAuth.swift`) as an alternative to an Auth0
bearer token. `admin-web` uses this exclusively - it never holds an Auth0 access token (it reads
`auth-web`'s shared session instead), so it can't present one to `/authz/check`'s or
`RolesController`'s usual bearer-token path. Whoever holds this shared secret is trusted
outright, on the assumption that `admin-web`'s own `AuthRequiredMiddleware` already verified the
acting user has the `admin` role before ever making the call - `RolesController` doesn't
re-derive who the acting admin is. `INTERNAL_SERVICE_TOKEN` and `admin-web`'s
`USERS_API_INTERNAL_SERVICE_TOKEN` must be the exact same value - both sides' `kubernetes/`
manifests pull from the one Akeyless path `/sweetrpg/admin/web/users-api` (see
`kubernetes/overlays/dev/secrets.yaml`), so there is only one secret to rotate, not two that
could drift apart.

### Audit logging (fail-closed, before and after)

Every mutating `/api/admin/*` route (`addRole`, `removeRole`, `addDenyEntry`,
`removeDenyEntry`) writes an `AdminActionAuditLog` row via `RolesController.performAudited`
before running the mutation, and updates that same row to `.succeeded`/`.failed` after. If the
pre-action write itself fails, the mutation never runs - an admin action that can't be logged is
not performed, no exceptions. The post-action update is best-effort (a failure there is logged
as a warning but doesn't undo an action that already happened - the pre-write is the hard gate).

Internal-service-token callers must also send `X-Acting-User-Sub` (the acting admin's Auth0
`sub`, resolved by `admin-web` from the shared session) - `verifyAdminRole` rejects the request
with 400 before touching the database if it's missing or empty, since the audit log needs to
know who to attribute the action to. Auth0 bearer-token callers don't need this header; their
own verified token's subject is used instead.

## Deployment

`kubernetes/` (base + `overlays/{dev,local}`) deploys this service into the `sweetrpg-users`
namespace as `api-v1`, server-to-server only - no Ingress, since every caller (`auth-web`,
`admin-web`, `users-web`) reaches it over in-cluster DNS
(`api-v1.sweetrpg-users.svc.cluster.local:8080`), matching the pattern documented in
`admin-web`'s own overlay comments for `ADMIN_API_URL`/`USERS_API_URL`. `DATABASE_URL`
(MongoDB), `AUTH0_DOMAIN`/`AUTH0_AUDIENCE` (the one shared Auth0 application - see
`auth-web`'s `AGENTS.md`), and `INTERNAL_SERVICE_TOKEN` all come from Akeyless via
`ExternalSecret`s, not the configmap. This service's own dedicated Redis instance
(`redis.sweetrpg-users.svc.cluster.local`) is its own Vapor session store, registered in
`sweetrpg/platform`'s `docs/frontend-conventions.md`, unrelated to auth-web's suite-wide shared
session in `sweetrpg-auth`, which this service never reads or writes.

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
