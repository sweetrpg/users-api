//
// AuthzController.swift
//

import Fluent
import UserModel
import Vapor

struct AuthzCheckRequest: Content {
  let service: String
  let action: String?
}

struct AuthzAllowedResponse: Content {
  let allowed: Bool
  let roles: [String]
  let sub: String
}

struct AuthzDeniedResponse: Content {
  let allowed: Bool
  let reason: String
}

struct AuthzErrorResponse: Content {
  let error: String
}

struct AuthzController: RouteCollection {
  static let endpointPath: PathComponent = "authz"

  func boot(routes: RoutesBuilder) throws {
    let group = routes.grouped(Self.endpointPath)
    group.post("check", use: self.check)
  }

  func check(req: Request) throws -> EventLoopFuture<Response> {
    let checkRequest = try req.content.decode(AuthzCheckRequest.self)

    guard let token = req.headers.bearerAuthorization?.token else {
      return self.invalidTokenResponse(req)
    }

    return req.verifyAuth0Token(token)
      .flatMap { payload in
        UserProvisioning.findOrCreateUser(subject: payload.subject.value, on: req.db)
          .flatMapThrowing { user -> (User, String) in
            (user, payload.subject.value)
          }
      }
      .flatMap { user, subject in
        self.authorize(user: user, service: checkRequest.service, subject: subject, req: req)
      }
      .flatMapError { _ in
        self.invalidTokenResponse(req)
      }
  }

  private func authorize(user: User, service: String, subject: String, req: Request)
    -> EventLoopFuture<Response>
  {
    guard let userId = user.id else {
      return req.eventLoop.makeFailedFuture(Abort(.internalServerError))
    }

    let rolesFuture = UserRole.query(on: req.db)
      .filter(\.$user.$id == userId)
      .all()
    let denyFuture = ServiceDenyEntry.query(on: req.db)
      .filter(\.$user.$id == userId)
      .filter(\.$service == service)
      .first()

    return rolesFuture.and(denyFuture).flatMapThrowing { roles, deny in
      let response = Response(status: .ok)
      if deny != nil {
        try response.content.encode(AuthzDeniedResponse(allowed: false, reason: "service_denied"))
        return response
      }
      let roleNames = roles.isEmpty ? [Role.user.rawValue] : roles.map { $0.role.rawValue }
      try response.content.encode(
        AuthzAllowedResponse(allowed: true, roles: roleNames, sub: subject))
      return response
    }
  }

  private func invalidTokenResponse(_ req: Request) -> EventLoopFuture<Response> {
    let response = Response(status: .unauthorized)
    do {
      try response.content.encode(AuthzErrorResponse(error: "invalid_token"))
    } catch {
      return req.eventLoop.makeFailedFuture(error)
    }
    return req.eventLoop.makeSucceededFuture(response)
  }
}
