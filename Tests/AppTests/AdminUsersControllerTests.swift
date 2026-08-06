//
// AdminUsersControllerTests.swift
//

import XCTVapor

@testable import App

/// Doesn't call `configure(_:)` (which requires a reachable MongoDB via `DATABASE_URL`) - only
/// exercises the internal-service-token gate, which runs before any database access, same
/// constraint as `InternalServiceAuthTests`.
final class AdminUsersControllerTests: XCTestCase {
  func testMissingInternalTokenIsUnauthorized() throws {
    let app = Application(.testing)
    defer { app.shutdown() }
    app.internalServiceToken = "test-internal-token"
    try app.register(collection: AdminUsersController())

    try app.testable().test(
      .GET, "api/admin/users",
      afterResponse: { res in
        XCTAssertEqual(res.status, .unauthorized)
      })
  }

  func testMismatchedInternalTokenIsUnauthorized() throws {
    let app = Application(.testing)
    defer { app.shutdown() }
    app.internalServiceToken = "test-internal-token"
    try app.register(collection: AdminUsersController())

    try app.testable().test(
      .GET, "api/admin/users",
      beforeRequest: { req in
        req.headers.replaceOrAdd(name: "X-Internal-Service-Token", value: "wrong-token")
      },
      afterResponse: { res in
        XCTAssertEqual(res.status, .unauthorized)
      })
  }
}
