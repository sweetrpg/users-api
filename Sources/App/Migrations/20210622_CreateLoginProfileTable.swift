//
// 20210620_CreateUserTable.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import UserModel

struct CreateLoginProfileTable: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    database.schema(LoginProfile.v20210620.schemaName)
      .id()
      .field(LoginProfile.v20210620.createdAt, .datetime, .required)
      .field(LoginProfile.v20210620.updatedAt, .datetime, .required)
      .field(LoginProfile.v20210620.deletedAt, .datetime)
      .field(LoginProfile.v20210620.username, .string, .required)
      .field(LoginProfile.v20210620.thirdPartyAuth, .string)
      .field(LoginProfile.v20210620.thirdPartyAuthId, .string)
      .field(
        LoginProfile.v20210620.userId, .uuid,
        .references(User.v20210620.schemaName, User.v20210620.id)
      )
      .unique(on: LoginProfile.v20210620.username)
      .create()
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    database.schema(LoginProfile.v20210620.schemaName)
      .delete()
  }
}
