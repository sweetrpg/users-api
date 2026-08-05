//
// AdminUsersController.swift
//

import Fluent
import UserModel
import Vapor

struct UserIdentity: Content {
  let id: UUID
  let email: String
}

/// Minimal id/email listing for `admin-web`'s role/service-access management UI to compose
/// against `auth-api`'s role/deny-entry data, keyed by Auth0 subject on `auth-api`'s side and by
/// `LoginProfile` here (`sweetrpg/platform`'s `split-authz-into-auth-api` change design.md's
/// "Admin listing is composed in admin-web" decision - `auth-api` holds no profile data, so it
/// can't serve this itself). Deliberately narrower than the `RolesController.listUsers`/`getUser`
/// endpoints that used to live here (which also returned roles/denials) - this only ever returns
/// identity, never authorization data.
struct AdminUsersController: RouteCollection {
  static let endpointPath: PathComponent = "admin"

  func boot(routes: RoutesBuilder) throws {
    let group = routes.grouped(Constants.apiPath, Self.endpointPath)
    group.get("users", use: self.listUsers)
  }

  private func listUsers(req: Request) throws -> EventLoopFuture<[UserIdentity]> {
    guard req.hasValidInternalServiceToken else {
      return req.eventLoop.makeFailedFuture(Abort(.unauthorized))
    }
    return User.query(on: req.db)
      .all()
      .flatMapThrowing { users in
        try users.map { user in
          guard let id = user.id else { throw Abort(.internalServerError) }
          return UserIdentity(id: id, email: user.email)
        }
      }
  }
}
