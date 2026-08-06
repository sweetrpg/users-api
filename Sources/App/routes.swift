//
// routes.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Common
import Fluent
import Vapor

func routes(_ app: Application) throws {
  app.get("status", "ping") { req -> [String: String] in
    ["status": "ok", "hostname": Environment.get("HOSTNAME") ?? "unknown"]
  }

  try app.register(collection: UsersController())
  try app.register(collection: AuthController())
  try app.register(collection: AdminUsersController())
}
