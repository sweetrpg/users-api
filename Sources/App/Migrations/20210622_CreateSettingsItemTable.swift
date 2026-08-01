//
// 20210620_CreateUserTable.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import UserModel

struct CreateSettingsItemTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(SettingsItem.v20210622.schemaName)
      .id()
      .field(SettingsItem.v20210622.createdAt, .datetime, .required)
      .field(SettingsItem.v20210622.updatedAt, .datetime, .required)
      .field(SettingsItem.v20210622.deletedAt, .datetime)
      .field(SettingsItem.v20210622.name, .string, .required)
      .field(SettingsItem.v20210622.value, .string, .required)
      .field(
        SettingsItem.v20210622.userId, .uuid,
        .references(User.v20210620.schemaName, User.v20210620.id)
      )
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(SettingsItem.v20210622.schemaName)
      .delete()
  }
}
