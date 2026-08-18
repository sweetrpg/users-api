# SweetRPG User API

[![CI](https://github.com/sweetrpg/users-api/actions/workflows/ci.yaml/badge.svg)](https://github.com/sweetrpg/users-api/actions/workflows/ci.yaml)
[![Coverage](https://img.shields.io/endpoint?url=https://sweetrpg.github.io/users-api/coverage-badge.json)](https://sweetrpg.github.io/users-api/)
[![License](https://img.shields.io/github/license/sweetrpg/users-api.svg)](https://img.shields.io/github/license/sweetrpg/users-api.svg)
[![Issues](https://img.shields.io/github/issues/sweetrpg/users-api.svg)](https://img.shields.io/github/issues/sweetrpg/users-api.svg)
[![PRs](https://img.shields.io/github/issues-pr/sweetrpg/users-api.svg)](https://img.shields.io/github/issues-pr/sweetrpg/users-api.svg)
[![Dependabot](https://badgen.net/github/dependabot/sweetrpg/users-api)](https://badgen.net/github/dependabot/sweetrpg/users-api)
[![Deployment](https://argocd.dev.pilgrimagesoftware.com/api/badge?name=sweetrpg-users-api&revision=true&showAppName=true&namespace=sweetrpg-system)](https://argocd.dev.pilgrimagesoftware.com/applications/sweetrpg-users-api)

[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
[![Built with love](https://ForTheBadge.com/images/badges/built-with-love.svg)](https://ForTheBadge.com/images/badges/built-with-love.svg)

Go API service for the platform's user profile/management data. Rewritten from Swift/Vapor - see
`sweetrpg/platform`'s `migrate-auth-users-api-to-go` OpenSpec change for the rationale.
Authentication/authorization moved to a dedicated `auth-api` service - see
`sweetrpg/platform`'s `split-authz-into-auth-api` OpenSpec change.

Exposes `GET /api/admin/users` for `admin-web`'s user listing. See `AGENTS.md` for the full
scope and consumer list.

## Run locally

```bash
go run cmd/users-api/main.go
```

Serves on `:8000` (`BIND_ADDRESS` to override).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and this repo's `AGENTS.md`
for the service's scope and consumers.
