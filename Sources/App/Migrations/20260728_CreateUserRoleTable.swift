//
// 20260728_CreateUserRoleTable.swift
//

import Fluent
import UserModel

struct CreateUserRoleTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(UserRole.v20260728.schemaName)
      .id()
      .field(UserRole.v20260728.createdAt, .datetime, .required)
      .field(UserRole.v20260728.role, .string, .required)
      .field(
        UserRole.v20260728.userId, .uuid,
        .references(User.v20210620.schemaName, User.v20210620.id)
      )
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(UserRole.v20260728.schemaName)
      .delete()
  }
}
