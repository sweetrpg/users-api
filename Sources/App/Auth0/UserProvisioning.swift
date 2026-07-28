//
// UserProvisioning.swift
//

import Fluent
import UserModel
import Vapor

/// Looks up (or provisions) the `User` for a verified Auth0 subject. A brand-new user gets no
/// `UserRole` rows at all - `Role.user` is the implicit default for anyone with no explicit
/// role assignment (see design.md's "Role model" decision), so there's nothing to insert here
/// beyond the user and its login-profile link.
enum UserProvisioning {
  static let auth0ThirdPartyAuth = "auth0"

  static func findOrCreateUser(subject: String, on db: Database) -> EventLoopFuture<User> {
    LoginProfile.query(on: db)
      .filter(\.$thirdPartyAuth == auth0ThirdPartyAuth)
      .filter(\.$thirdPartyAuthId == subject)
      .with(\.$user)
      .first()
      .flatMap { existing in
        if let existing = existing {
          return db.eventLoop.makeSucceededFuture(existing.user)
        }
        return self.createUser(subject: subject, on: db)
      }
  }

  private static func createUser(subject: String, on db: Database) -> EventLoopFuture<User> {
    let user = User(name: subject, email: "")
    return user.save(on: db).flatMap {
      guard let userId = user.id else {
        return db.eventLoop.makeFailedFuture(Abort(.internalServerError))
      }
      let loginProfile = LoginProfile(
        username: subject, thirdPartyAuth: auth0ThirdPartyAuth, thirdPartyAuthId: subject,
        userId: userId)
      return loginProfile.save(on: db).map { user }
    }
  }
}
