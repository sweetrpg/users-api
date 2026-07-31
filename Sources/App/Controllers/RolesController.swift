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
      .flatMapThrowing { _ in
        guard let userId = req.parameters.get("userId", as: UUID.self) else {
          throw Abort(.badRequest, reason: "Invalid user ID")
        }

        let request = try req.content.decode(RoleAssignmentRequest.self)

        guard let role = Role(rawValue: request.role) else {
          throw Abort(.badRequest, reason: "Invalid role")
        }

        return (userId, role)
      }
      .flatMap { userId, role in
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

  private func removeRole(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMapThrowing { _ in
        guard let userId = req.parameters.get("userId", as: UUID.self) else {
          throw Abort(.badRequest, reason: "Invalid user ID")
        }

        guard let roleString = req.parameters.get("role") else {
          throw Abort(.badRequest, reason: "Invalid role")
        }

        guard let role = Role(rawValue: roleString) else {
          throw Abort(.badRequest, reason: "Invalid role")
        }

        return (userId, role)
      }
      .flatMap { userId, role in
        UserRole.query(on: req.db)
          .filter(\.$user.$id == userId)
          .filter(\.$role == role)
          .delete()
          .map { _ in .noContent }
      }
  }

  private func addDenyEntry(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMapThrowing { _ in
        guard let userId = req.parameters.get("userId", as: UUID.self) else {
          throw Abort(.badRequest, reason: "Invalid user ID")
        }

        let request = try req.content.decode(DenyEntryRequest.self)
        return (userId, request.service)
      }
      .flatMap { userId, service in
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

  private func removeDenyEntry(req: Request) throws -> EventLoopFuture<HTTPStatus> {
    return self.verifyAdminRole(req: req)
      .flatMapThrowing { _ in
        guard let userId = req.parameters.get("userId", as: UUID.self) else {
          throw Abort(.badRequest, reason: "Invalid user ID")
        }

        guard let service = req.parameters.get("service") else {
          throw Abort(.badRequest, reason: "Invalid service")
        }

        return (userId, service)
      }
      .flatMap { userId, service in
        ServiceDenyEntry.query(on: req.db)
          .filter(\.$user.$id == userId)
          .filter(\.$service == service)
          .delete()
          .map { _ in .noContent }
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

  private func verifyAdminRole(req: Request) -> EventLoopFuture<Void> {
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
              .flatMapThrowing { roles in
                let hasAdminRole = roles.contains { $0.role == .admin }
                if !hasAdminRole {
                  throw Abort(.forbidden, reason: "Admin role required")
                }
              }
          }
      }
      .flatMapError { _ in
        req.eventLoop.makeFailedFuture(Abort(.unauthorized))
      }
  }
}
