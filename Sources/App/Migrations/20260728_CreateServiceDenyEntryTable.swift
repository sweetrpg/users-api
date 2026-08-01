//
// 20260728_CreateServiceDenyEntryTable.swift
//

import Fluent
import UserModel

struct CreateServiceDenyEntryTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(ServiceDenyEntry.v20260728.schemaName)
      .id()
      .field(ServiceDenyEntry.v20260728.createdAt, .datetime, .required)
      .field(ServiceDenyEntry.v20260728.service, .string, .required)
      .field(
        ServiceDenyEntry.v20260728.userId, .uuid,
        .references(User.v20210620.schemaName, User.v20210620.id)
      )
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(ServiceDenyEntry.v20260728.schemaName)
      .delete()
  }
}
