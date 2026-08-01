# SweetRPG User API

[![CI](https://github.com/sweetrpg/users-api/actions/workflows/ci.yaml/badge.svg)](https://github.com/sweetrpg/users-api/actions/workflows/ci.yaml)
[![Coverage](https://img.shields.io/endpoint?url=https://sweetrpg.github.io/users-api/coverage-badge.json)](https://sweetrpg.github.io/users-api/)
[![License](https://img.shields.io/github/license/sweetrpg/users-api.svg)](https://img.shields.io/github/license/sweetrpg/users-api.svg)
[![Issues](https://img.shields.io/github/issues/sweetrpg/users-api.svg)](https://img.shields.io/github/issues/sweetrpg/users-api.svg)
[![PRs](https://img.shields.io/github/issues-pr/sweetrpg/users-api.svg)](https://img.shields.io/github/issues-pr/sweetrpg/users-api.svg)
[![Dependabot](https://badgen.net/github/dependabot/sweetrpg/users-api)](https://badgen.net/github/dependabot/sweetrpg/users-api)
[![Deployment](https://argocd.dev.pilgrimagesoftware.com/api/badge?name=sweetrpg-users-api&revision=true&showAppName=true&namespace=sweetrpg-system)](https://argocd.dev.pilgrimagesoftware.com/applications/sweetrpg-users-api)

[![Swift](https://img.shields.io/badge/Swift-F05138?style=for-the-badge&logo=swift&logoColor=white)](https://img.shields.io/badge/Swift-F05138?style=for-the-badge&logo=swift&logoColor=white)
[![Built with love](https://ForTheBadge.com/images/badges/built-with-love.svg)](https://ForTheBadge.com/images/badges/built-with-love.svg)

Vapor (Swift) API service for the platform's identity domain: authentication, authorization,
profile data, settings, and entitlements. Renamed from `profiles-api` - see `sweetrpg/platform`'s
`add-user-api-authn-authz` OpenSpec change for the rationale.

Verifies Auth0-issued access tokens server-side (JWKS signature verification) and exposes
`POST /authz/check` for other services to call. See `AGENTS.md` for the role model and consumer
list.

## Run locally

```bash
swift run
```

Serves on `:8080`. Without `REDIS_HOST` set, falls back to in-memory sessions and no response
caching - fine for local development.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and this repo's `AGENTS.md`
for the service's scope and consumers.
