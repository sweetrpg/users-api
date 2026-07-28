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

- `users-web`: the admin-gated role/service-access management UI, and end-user profile/settings
  pages.
- `main-web`: calls `/authz/check` after Auth0 login to establish a session's verified roles.
- `catalog-web`: has its own separate, unverified local Auth0 ID token decode for display
  purposes only - retrofitting it to call this service is an explicit follow-up, not part of
  this change.

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
