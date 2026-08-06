//
// AdminUsersController.swift
//

import Fluent
import UserModel
import Vapor

struct UserIdentity: Content {
  let id: UUID
  let email: String
  /// The Auth0 `sub` this user last logged in with (from `LoginProfile`), the key `auth-api`'s
  /// role/deny-entry data is stored under - `nil` if this user has no Auth0 `LoginProfile` yet
  /// (shouldn't happen in practice since Auth0 is the platform's sole login mechanism, but not
  /// guaranteed by the schema). `admin-web` composes against `auth-api` using this field, not
  /// `id` - see `sweetrpg/platform`'s `split-authz-into-auth-api` change design.md.
  let subject: String?
}

/// Minimal id/email/subject listing for `admin-web`'s role/service-access management UI to
/// compose against `auth-api`'s role/deny-entry data (`auth-api` holds no profile data, so it
/// can't serve this itself - see the "Admin listing is composed in admin-web" design decision).
/// Deliberately narrower than the `RolesController.listUsers`/`getUser` endpoints that used to
/// live here (which also returned roles/denials) - this only ever returns identity, never
/// authorization data.
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
      .and(
        LoginProfile.query(on: req.db)
          .filter(\.$thirdPartyAuth == "auth0")
          .all()
      )
      .flatMapThrowing { users, loginProfiles in
        let subjectsByUserId = Dictionary(
          loginProfiles.map { ($0.$user.id, $0.thirdPartyAuthId) },
          uniquingKeysWith: { first, _ in first })
        return try users.map { user in
          guard let id = user.id else { throw Abort(.internalServerError) }
          return UserIdentity(id: id, email: user.email, subject: subjectsByUserId[id])
        }
      }
  }
}
