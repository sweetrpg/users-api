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
