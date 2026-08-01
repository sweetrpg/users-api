//
// 20260731_CreateAdminActionAuditLogTable.swift
//

import Fluent

struct CreateAdminActionAuditLogTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(AdminActionAuditLog.schema)
      .id()
      .field("actingUserSub", .string, .required)
      .field("action", .string, .required)
      .field("targetUserId", .uuid, .required)
      .field("detail", .string, .required)
      .field("status", .string, .required)
      .field("attemptedAt", .datetime, .required)
      .field("completedAt", .datetime)
      .field("errorMessage", .string)
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(AdminActionAuditLog.schema)
      .delete()
  }
}
