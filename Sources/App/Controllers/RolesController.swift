//
// RolesController.swift
//

import Fluent
import UserModel
import Vapor

struct UserSummary: Content {
  let id: UUID
  let email: String
  let roles: [String]
  let deniedServices: [String]
}

struct RoleAssignmentRequest: Content {
  let role: String
}

struct DenyEntryRequest: Content {
  let service: String
}

struct RolesController: RouteCollection {
  static let endpointPath: PathComponent = "admin"

  func boot(routes: RoutesBuilder) throws {
    let group = routes.grouped(Constants.apiPath, Self.endpointPath)

    // List all users with their roles and deny entries
    group.get("users", use: self.listUsers)

    // Get a specific user's roles and deny entries
    group.get("users", ":userId", use: self.getUser)

    // Add a role to a user
    group.post("users", ":userId", "roles", use: self.addRole)

    // Remove a role from a user
    group.delete("users", ":userId", "roles", ":role", use: self.removeRole)

    // Add a service deny entry for a user
    group.post("users", ":userId", "deny-entries", use: self.addDenyEntry)

    // Remove a service deny entry for a user
    group.delete("users", ":userId", "deny-entries", ":service", use: self.removeDenyEntry)
  }

  private func listUsers(req: Request) throws -> EventLoopFuture<[UserSummary]> {
    return self.verifyAdminRole(req: req)
      .flatMap { _ in
        User.query(on: req.db)
          .all()
          .flatMapEach(on: req.eventLoop) { user in
            self.buildUserSummary(user: user, on: req)
          }
      }
  }

  private func getUser(req: Request) throws -> EventLoopFuture<UserSummary> {
    return self.verifyAdminRole(req: req)
      .flatMapThrowing { _ in
        guard let userId = req.parameters.get("userId", as: UUID.self) else {
          throw Abort(.badRequest, reason: "Invalid user ID")
        }
        return userId
      }
      .flatMap { userId in
        User.find(userId, on: req.db)
          .unwrap(or: Abort(.notFound))
          .flatMap { user in
            self.buildUserSummary(user: user, on: req)
          }
      }
  }

  private func addRole(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMap { actingUserSub in
        req.eventLoop.makeCompletedFuture {
          guard let userId = req.parameters.get("userId", as: UUID.self) else {
            throw Abort(.badRequest, reason: "Invalid user ID")
          }
          let request = try req.content.decode(RoleAssignmentRequest.self)
          guard let role = Role(rawValue: request.role) else {
            throw Abort(.badRequest, reason: "Invalid role")
          }
          return (actingUserSub, userId, role)
        }
      }
      .flatMap { actingUserSub, userId, role in
        self.performAudited(
          req: req, actingUserSub: actingUserSub, action: "add_role", targetUserId: userId,
          detail: role.rawValue
        ) {
          // Check if the role already exists
          UserRole.query(on: req.db)
            .filter(\.$user.$id == userId)
            .filter(\.$role == role)
            .first()
            .flatMap { existing in
              if existing != nil {
                // Role already assigned
                return req.eventLoop.makeSucceededFuture(.noContent)
              }

              let userRole = UserRole(userId: userId, role: role)
              return userRole.save(on: req.db)
                .map { .created }
            }
        }
      }
  }

  private func removeRole(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMap { actingUserSub in
        req.eventLoop.makeCompletedFuture {
          guard let userId = req.parameters.get("userId", as: UUID.self) else {
            throw Abort(.badRequest, reason: "Invalid user ID")
          }
          guard let roleString = req.parameters.get("role"),
            let role = Role(rawValue: roleString)
          else {
            throw Abort(.badRequest, reason: "Invalid role")
          }
          return (actingUserSub, userId, role)
        }
      }
      .flatMap { actingUserSub, userId, role in
        self.performAudited(
          req: req, actingUserSub: actingUserSub, action: "remove_role", targetUserId: userId,
          detail: role.rawValue
        ) {
          UserRole.query(on: req.db)
            .filter(\.$user.$id == userId)
            .filter(\.$role == role)
            .delete()
            .map { _ in .noContent }
        }
      }
  }

  private func addDenyEntry(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMap { actingUserSub in
        req.eventLoop.makeCompletedFuture {
          guard let userId = req.parameters.get("userId", as: UUID.self) else {
            throw Abort(.badRequest, reason: "Invalid user ID")
          }
          let request = try req.content.decode(DenyEntryRequest.self)
          return (actingUserSub, userId, request.service)
        }
      }
      .flatMap { actingUserSub, userId, service in
        self.performAudited(
          req: req, actingUserSub: actingUserSub, action: "add_deny_entry",
          targetUserId: userId, detail: service
        ) {
          // Check if the deny entry already exists
          ServiceDenyEntry.query(on: req.db)
            .filter(\.$user.$id == userId)
            .filter(\.$service == service)
            .first()
            .flatMap { existing in
              if existing != nil {
                // Deny entry already exists
                return req.eventLoop.makeSucceededFuture(.noContent)
              }

              let denyEntry = ServiceDenyEntry(userId: userId, service: service)
              return denyEntry.save(on: req.db)
                .map { .created }
            }
        }
      }
  }

  private func removeDenyEntry(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMap { actingUserSub in
        req.eventLoop.makeCompletedFuture {
          guard let userId = req.parameters.get("userId", as: UUID.self) else {
            throw Abort(.badRequest, reason: "Invalid user ID")
          }
          guard let service = req.parameters.get("service") else {
            throw Abort(.badRequest, reason: "Invalid service")
          }
          return (actingUserSub, userId, service)
        }
      }
      .flatMap { actingUserSub, userId, service in
        self.performAudited(
          req: req, actingUserSub: actingUserSub, action: "remove_deny_entry",
          targetUserId: userId, detail: service
        ) {
          ServiceDenyEntry.query(on: req.db)
            .filter(\.$user.$id == userId)
            .filter(\.$service == service)
            .delete()
            .map { _ in .noContent }
        }
      }
  }

  /// Writes an `AdminActionAuditLog` row *before* running `operation`, and fails closed - never
  /// calling `operation` at all - if that write itself fails: an admin action that can't be
  /// logged must not be performed. Updates the same row to `.succeeded`/`.failed` after
  /// `operation` completes; that post-write is best-effort (logged on failure, but doesn't
  /// retroactively undo an action that already happened - the pre-write is the hard gate, not
  /// the post-write).
  private func performAudited(
    req: Request, actingUserSub: String, action: String, targetUserId: UUID, detail: String,
    operation: @escaping () -> EventLoopFuture<HTTPStatus>
  ) -> EventLoopFuture<HTTPStatus> {
    let log = AdminActionAuditLog(
      actingUserSub: actingUserSub, action: action, targetUserId: targetUserId, detail: detail)

    return log.save(on: req.db)
      .flatMapError { error in
        req.logger.error(
          "Refusing to perform admin action \(action) - audit log write failed: \(error)")
        return req.eventLoop.makeFailedFuture(
          Abort(.internalServerError, reason: "Could not record audit log; action not performed"))
      }
      .flatMap { _ -> EventLoopFuture<HTTPStatus> in
        operation()
          .flatMap { status in
            log.status = .succeeded
            log.completedAt = Date()
            return log.save(on: req.db)
              .flatMapErrorThrowing { error in
                req.logger.warning(
                  "Failed to record success for audit log \(log.id?.uuidString ?? "?"): \(error)"
                )
              }
              .map { status }
          }
          .flatMapError { error in
            log.status = .failed
            log.completedAt = Date()
            log.errorMessage = String(describing: error)
            return log.save(on: req.db)
              .flatMapErrorThrowing { saveError in
                req.logger.warning(
                  "Failed to record failure for audit log \(log.id?.uuidString ?? "?"): \(saveError)"
                )
              }
              .flatMap { req.eventLoop.makeFailedFuture(error) }
          }
      }
  }

  private func buildUserSummary(user: User, on req: Request) -> EventLoopFuture<UserSummary> {
    guard let userId = user.id else {
      return req.eventLoop.makeFailedFuture(Abort(.internalServerError))
    }

    let rolesFuture = UserRole.query(on: req.db)
      .filter(\.$user.$id == userId)
      .all()

    let denialsFuture = ServiceDenyEntry.query(on: req.db)
      .filter(\.$user.$id == userId)
      .all()

    return rolesFuture.and(denialsFuture).map { roles, denials in
      let roleNames = roles.isEmpty ? [Role.user.rawValue] : roles.map { $0.role.rawValue }
      let deniedServices = denials.map { $0.service }

      return UserSummary(
        id: userId,
        email: user.email,
        roles: roleNames,
        deniedServices: deniedServices
      )
    }
  }

  /// Trusts a valid `X-Internal-Service-Token` outright (see `InternalServiceAuth.swift` for
  /// why `admin-web` uses this instead of an Auth0 bearer token) - but still requires an
  /// `X-Acting-User-Sub` header identifying who initiated the action, since every mutating
  /// route needs that for its audit log (see `performAudited`). Falls through to verifying an
  /// Auth0 bearer token's `admin` role otherwise, using the verified token's own subject as the
  /// acting user. Returns the resolved acting user's `sub` on success.
  private func verifyAdminRole(req: Request) -> EventLoopFuture<String> {
    if req.hasValidInternalServiceToken {
      guard let actingUserSub = req.headers.first(name: "X-Acting-User-Sub"), !actingUserSub.isEmpty
      else {
        return req.eventLoop.makeFailedFuture(
          Abort(.badRequest, reason: "X-Acting-User-Sub header is required"))
      }
      return req.eventLoop.makeSucceededFuture(actingUserSub)
    }

    guard let token = req.headers.bearerAuthorization?.token else {
      return req.eventLoop.makeFailedFuture(Abort(.unauthorized))
    }

    return req.verifyAuth0Token(token)
      .flatMap { payload in
        UserProvisioning.findOrCreateUser(subject: payload.subject.value, on: req.db)
          .flatMapThrowing { user -> UUID in
            guard let userId = user.id else {
              throw Abort(.internalServerError)
            }
            return userId
          }
          .flatMap { userId in
            UserRole.query(on: req.db)
              .filter(\.$user.$id == userId)
              .all()
              .flatMapThrowing { roles -> String in
                let hasAdminRole = roles.contains { $0.role == .admin }
                if !hasAdminRole {
                  throw Abort(.forbidden, reason: "Admin role required")
                }
                return payload.subject.value
              }
          }
      }
      .flatMapError { _ in
        req.eventLoop.makeFailedFuture(Abort(.unauthorized))
      }
  }
}
