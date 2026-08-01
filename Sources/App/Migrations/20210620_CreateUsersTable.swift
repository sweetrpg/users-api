//
// 20210620_CreateUserTable.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import UserModel

struct CreateUserTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(User.v20210620.schemaName)
      .id()
      .field(User.v20210620.createdAt, .datetime, .required)
      .field(User.v20210620.updatedAt, .datetime, .required)
      .field(User.v20210620.deletedAt, .datetime)
      .field(User.v20210620.name, .string, .required)
      .field(User.v20210620.email, .string, .required)
      .unique(on: User.v20210620.email)
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(User.v20210620.schemaName)
      .delete()
  }
}
